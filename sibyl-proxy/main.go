package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ========================================================================
// KONFIGURASI
// ========================================================================

const (
	proxyListenAddr = ":4000"
	cacheTTL        = 300 * time.Second       // 5 menit — masa berlaku cache IP aman.
	evalTimeout     = 1500 * time.Millisecond  // Batas waktu evaluasi kognitif (TDD §1.3).
	syncInterval    = 15 * time.Second         // Interval polling blacklist dari Cloud Run (Opsi B).
	maxBodySize     = 2 * 1024 * 1024          // 2MB — proteksi heap (TDD §1.3).
	ccThreshold     = 75                       // Crime Coefficient >= 75 = BLOKIR.
)

// ========================================================================
// PROMETHEUS METRICS
// ========================================================================

var (
	proxyRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_requests_total",
		Help: "Total request yang diproses oleh Sibyl-Proxy.",
	}, []string{"decision"})
	proxyEvalLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "proxy_eval_latency_seconds",
		Help:    "Latensi round-trip evaluasi payload ke Sibyl-Brain.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 0.75, 1.0, 1.5, 2.0},
	})
)

func init() {
	prometheus.MustRegister(proxyRequestsTotal)
	prometheus.MustRegister(proxyEvalLatency)
}

// ========================================================================
// KONTRAK DATA — Harus identik dengan Sibyl-Brain
// ========================================================================

type EvalRequest struct {
	ClientIP string            `json:"client_ip"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Payload  string            `json:"payload"`
}

type EvalResponse struct {
	CrimeCoefficient int    `json:"crime_coefficient"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
}

type BlacklistResponse struct {
	BannedIPs []string `json:"banned_ips"`
	Count     int      `json:"count"`
}

// ========================================================================
// STATE — In-Memory (Thread-Safe via sync.Map)
// ========================================================================

// localCache menyimpan IP yang sudah divalidasi AMAN oleh Gemini.
// Key: string (IP), Value: time.Time (waktu kedaluwarsa cache).
var localCache sync.Map

// globalBlacklist menyimpan IP yang dinyatakan BAHAYA.
// Key: string (IP), Value: bool (true = banned).
var globalBlacklist sync.Map

// bufferPool mendaur ulang array byte untuk membaca body request (Zero-Allocation).
// Menghindari GC cycle berat saat menerima trafik ekstrem.
var bufferPool = sync.Pool{
	New: func() any {
		b := make([]byte, maxBodySize)
		return &b
	},
}

// brainBaseURL adalah alamat Sibyl-Brain (Cloud Run).
// Diambil dari env SIBYL_BRAIN_URL, default ke localhost:8080 untuk dev lokal.
var brainBaseURL string

// ========================================================================
// ENTRY POINT
// ========================================================================

func main() {
	// Setup structured logging — JSON ke stdout.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	targetAppAddr := os.Getenv("TARGET_APP_URL")
	if targetAppAddr == "" {
		targetAppAddr = "http://localhost:3000"
	}

	brainBaseURL = os.Getenv("SIBYL_BRAIN_URL")
	if brainBaseURL == "" {
		brainBaseURL = "http://localhost:8080"
	}

	targetURL, err := url.Parse(targetAppAddr)
	if err != nil {
		slog.Error("URL target tidak valid", "error", err)
		os.Exit(1)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Override ErrorHandler agar proxy tidak panic saat target mati.
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		slog.Error("target unreachable", "error", err)
		http.Error(w, `{"error":"target application tidak merespons"}`, http.StatusBadGateway)
	}

	// Jalankan goroutine sinkronisasi blacklist (CDN-Backed Polling / Opsi B).
	go startBlacklistSyncLoop()

	// Jalankan goroutine pembersihan cache kedaluwarsa.
	go startCacheEvictionLoop()

	mux := http.NewServeMux()

	// /metrics WAJIB di-register di sini (layer statis) agar TIDAK
	// ter-intercept oleh logika isPrivatePath() yang mencegat /api/* dan /rest/*.
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, proxy)
	})

	slog.Info("SIBYL-PROXY: starting",
		"listen", proxyListenAddr,
		"target", targetAppAddr,
		"brain", brainBaseURL,
	)
	if err := http.ListenAndServe(proxyListenAddr, mux); err != nil {
		slog.Error("server gagal start", "error", err)
		os.Exit(1)
	}
}

// ========================================================================
// REQUEST HANDLER — Multi-Layer Decision Engine
// ========================================================================

func handleRequest(w http.ResponseWriter, r *http.Request, proxy *httputil.ReverseProxy) {
	clientIP := extractClientIP(r)
	path := r.URL.Path

	// --- LAYER 0: Path Filter ---
	// Hanya intercept rute privat/sensitif (/api/*, /rest/*).
	// Rute statis/publik langsung bypass tanpa evaluasi kognitif.
	if !isPrivatePath(path) {
		proxyRequestsTotal.WithLabelValues("bypass").Inc()
		proxy.ServeHTTP(w, r)
		return
	}

	// --- LAYER 1: Cek Global Blacklist (Tercepat) ---
	if _, isBanned := globalBlacklist.Load(clientIP); isBanned {
		proxyRequestsTotal.WithLabelValues("blacklist_hit").Inc()
		slog.Warn("blacklist hit",
			"client_ip", clientIP,
			"path", path,
		)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprintf(w, `{"error":"HTTP 403 Forbidden: Evaluasi Kognitif Mendeteksi Ancaman Persisten.","client_ip":"%s"}`, clientIP)
		return
	}

	// --- LAYER 2: Cek Local Cache (IP sudah divalidasi AMAN) ---
	if expiry, isCached := localCache.Load(clientIP); isCached {
		if t, ok := expiry.(time.Time); ok && time.Now().Before(t) {
			// Cache masih valid — bypass evaluasi.
			proxyRequestsTotal.WithLabelValues("cache_hit").Inc()
			proxy.ServeHTTP(w, r)
			return
		}
		// Cache kedaluwarsa — hapus dan lanjut ke evaluasi.
		localCache.Delete(clientIP)
	}

	// --- EKSTRAKSI PAYLOAD (MEMORY OPTIMIZATION) ---
	var bodyStr string
	if r.Body != nil {
		// Pinjam buffer dari memori menggunakan sync.Pool dengan tipe pointer presisi
		bufPtr := bufferPool.Get().(*[]byte)
		// Pastikan buffer direset dan dikembalikan ke pool (setelah proxy.ServeHTTP selesai)
		defer bufferPool.Put(bufPtr)

		buffer := *bufPtr
		// Baca body ke dalam buffer menggunakan io.LimitReader
		n, err := io.ReadFull(io.LimitReader(r.Body, maxBodySize), buffer)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			slog.Error("gagal baca request body", "error", err)
			http.Error(w, `{"error":"gagal baca body"}`, http.StatusBadRequest)
			return
		}

		bodyStr = string(buffer[:n])
		// Kembalikan isinya ke request aslinya menggunakan io.NopCloser
		r.Body = io.NopCloser(bytes.NewReader(buffer[:n]))
	}

	// --- LAYER 3: Evaluasi Kognitif via Sibyl-Brain ---
	evalStart := time.Now()
	evalResult, err := evaluatePayload(r, clientIP, bodyStr)
	evalDuration := time.Since(evalStart).Seconds()
	proxyEvalLatency.Observe(evalDuration)

	if err != nil {
		// Fail-Closed: AI gagal/timeout → tolak request (TDD §1.3).
		proxyRequestsTotal.WithLabelValues("fail_closed").Inc()
		slog.Error("evaluasi gagal, fail-closed",
			"client_ip", clientIP,
			"path", path,
			"latency_s", evalDuration,
			"error", err,
		)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"error":"HTTP 503: Mesin kognitif tidak merespons. Koneksi diputus demi keamanan.","client_ip":"%s"}`, clientIP)
		return
	}

	if evalResult.CrimeCoefficient >= ccThreshold {
		// BAHAYA — blokir dan masukkan ke blacklist lokal instan.
		globalBlacklist.Store(clientIP, true)
		proxyRequestsTotal.WithLabelValues("blocked").Inc()
		slog.Warn("BLOKIR",
			"client_ip", clientIP,
			"path", path,
			"crime_coefficient", evalResult.CrimeCoefficient,
			"reason", evalResult.Reason,
			"latency_s", evalDuration,
		)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "HTTP 403 Forbidden: Sibyl-WAF Mendeteksi Niat Jahat.",
			"crime_coefficient": evalResult.CrimeCoefficient,
			"reason":            evalResult.Reason,
			"client_ip":         clientIP,
		})
		return
	}

	// AMAN — cache IP dan teruskan ke target.
	localCache.Store(clientIP, time.Now().Add(cacheTTL))
	proxyRequestsTotal.WithLabelValues("passed").Inc()
	slog.Info("AMAN",
		"client_ip", clientIP,
		"path", path,
		"crime_coefficient", evalResult.CrimeCoefficient,
		"latency_s", evalDuration,
	)
	proxy.ServeHTTP(w, r)
}

// ========================================================================
// EVALUASI PAYLOAD — HTTP POST ke Sibyl-Brain
// ========================================================================

func evaluatePayload(r *http.Request, clientIP string, bodyStr string) (*EvalResponse, error) {
	// Ekstrak headers ke flat map.
	headerMap := make(map[string]string, len(r.Header))
	for key, vals := range r.Header {
		headerMap[key] = vals[0]
	}

	evalReq := EvalRequest{
		ClientIP: clientIP,
		Method:   r.Method,
		Path:     r.URL.Path,
		Headers:  headerMap,
		Payload:  bodyStr,
	}

	reqBody, err := json.Marshal(evalReq)
	if err != nil {
		return nil, fmt.Errorf("gagal marshal eval request: %w", err)
	}

	// Context timeout ketat 1500ms (TDD §1.3).
	ctx, cancel := context.WithTimeout(context.Background(), evalTimeout)
	defer cancel()

	evalURL := brainBaseURL + "/api/v1/eval"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, evalURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("gagal buat HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP POST ke brain gagal: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Baca response body dengan limit proteksi.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("gagal baca response brain: %w", err)
	}

	// Jika Brain mengembalikan 503 (Fail-Closed dari sisi Brain), propagate error.
	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, fmt.Errorf("brain mengembalikan 503: %s", string(respBody))
	}

	var evalResp EvalResponse
	if err := json.Unmarshal(respBody, &evalResp); err != nil {
		return nil, fmt.Errorf("gagal parse response brain: %w (raw: %s)", err, string(respBody))
	}

	return &evalResp, nil
}

// ========================================================================
// CDN-BACKED POLLING — Sinkronisasi Blacklist dari Cloud Run (Opsi B)
// ========================================================================

func startBlacklistSyncLoop() {
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	slog.Info("blacklist sync loop dimulai", "interval", syncInterval.String())

	// Eksekusi langsung saat startup, jangan tunggu tick pertama.
	syncBlacklist()

	for range ticker.C {
		syncBlacklist()
	}
}

func syncBlacklist() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blacklistURL := brainBaseURL + "/api/v1/blacklist"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blacklistURL, nil)
	if err != nil {
		slog.Warn("sync: gagal buat request", "error", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("sync: gagal fetch blacklist", "error", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("sync: brain status non-OK", "status_code", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		slog.Warn("sync: gagal baca body", "error", err)
		return
	}

	var blResp BlacklistResponse
	if err := json.Unmarshal(body, &blResp); err != nil {
		slog.Warn("sync: gagal parse JSON", "error", err)
		return
	}

	// Merge: tambahkan IP baru ke globalBlacklist tanpa menghapus yang sudah ada.
	// Ini additive-only — IP yang sudah diblokir lokal tetap diblokir.
	newCount := 0
	for _, ip := range blResp.BannedIPs {
		if _, exists := globalBlacklist.Load(ip); !exists {
			globalBlacklist.Store(ip, true)
			newCount++
		}
	}

	if newCount > 0 {
		slog.Info("blacklist diperbarui", "new_ips", newCount, "total_from_brain", blResp.Count)
	}
}

// ========================================================================
// CACHE EVICTION — Pembersihan Entri Kedaluwarsa
// ========================================================================

func startCacheEvictionLoop() {
	ticker := time.NewTicker(60 * time.Second) // Sweep setiap 60 detik.
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		evicted := 0
		localCache.Range(func(key, value any) bool {
			if t, ok := value.(time.Time); ok && now.After(t) {
				localCache.Delete(key)
				evicted++
			}
			return true
		})
		if evicted > 0 {
			slog.Info("cache eviction", "evicted", evicted)
		}
	}
}

// ========================================================================
// UTILITAS
// ========================================================================

// isPrivatePath menentukan apakah path harus dicegat oleh WAF.
// Hanya /api/* dan /rest/* yang perlu evaluasi kognitif.
func isPrivatePath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/rest/")
}

// extractClientIP mengambil IP klien asli dari header yang di-set oleh Nginx.
// Urutan prioritas: X-Real-IP → X-Forwarded-For (entry pertama) → RemoteAddr.
func extractClientIP(r *http.Request) string {
	// Prioritas 1: Header dari Nginx.
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Prioritas 2: X-Forwarded-For (bisa berisi chain, ambil yang pertama).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}

	// Prioritas 3: Direct connection (strip port).
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
