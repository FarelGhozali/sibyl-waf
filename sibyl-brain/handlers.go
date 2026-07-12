package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/valkey-io/valkey-go"
	"google.golang.org/api/option"
)

// ========================================================================
// KONTRAK DATA (TDD §1.4)
// ========================================================================

// EvalRequest — Payload yang dikirim oleh Sibyl-Proxy untuk evaluasi kognitif.
type EvalRequest struct {
	ClientIP string            `json:"client_ip"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers"`
	Payload  string            `json:"payload"`
}

// EvalResponse — Putusan dari mesin kognitif (Gemini API) terhadap sebuah request.
type EvalResponse struct {
	CrimeCoefficient int    `json:"crime_coefficient"`
	Status           string `json:"status"` // "AMAN" | "BAHAYA"
	Reason           string `json:"reason"`
}

// ========================================================================
// STATE — Global Blacklist (In-Memory)
// ========================================================================

// globalBlacklist menyimpan IP yang sudah divonis berbahaya.
// Key: string (IP), Value: bool (true = banned).
// Thread-safe via sync.Map untuk akses konkuren dari handler + goroutine sinkronisasi.
var globalBlacklist sync.Map

// ========================================================================
// GEMINI CLIENT — Singleton
// ========================================================================

// geminiClient di-inisialisasi sekali saat startup via InitGeminiClient().
// Tidak boleh nil saat handler dipanggil. Fail-fast di main() jika init gagal.
var geminiClient *genai.Client

// InitGeminiClient membuat koneksi ke Gemini API menggunakan API Key dari env.
// Wajib dipanggil di main() sebelum server listen. Mengembalikan cleanup func.
func InitGeminiClient() func() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		slog.Error("GEMINI_API_KEY tidak di-set, abort")
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		slog.Error("gagal inisialisasi Gemini client", "error", err)
		os.Exit(1)
	}

	geminiClient = client
	slog.Info("Gemini client berhasil diinisialisasi")

	return func() {
		if err := geminiClient.Close(); err != nil {
			slog.Warn("gagal menutup Gemini client", "error", err)
		}
	}
}

// ========================================================================
// VALKEY CLIENT — Inisialisasi
// ========================================================================

var valkeyClient valkey.Client

// InitValkeyClient menginisialisasi koneksi ke Valkey.
// Jika gagal, log error dan lanjutkan tanpa panic (degraded mode).
func InitValkeyClient() {
	valkeyAddr := os.Getenv("VALKEY_URL")
	if valkeyAddr == "" {
		valkeyAddr = "127.0.0.1:6379"
	}
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{valkeyAddr}})
	if err != nil {
		slog.Warn("gagal koneksi ke Valkey, mode degraded", "error", err, "addr", valkeyAddr)
	} else {
		valkeyClient = client
		slog.Info("Valkey client berhasil diinisialisasi", "addr", valkeyAddr)
	}
}

// SeedMockData menyuntikkan data log fiktif jika Valkey kosong.
func SeedMockData() {
	if valkeyClient == nil {
		return
	}
	ctx := context.Background()
	exists, _ := valkeyClient.Do(ctx, valkeyClient.B().Exists().Key("metric:total_analyzed").Build()).AsInt64()
	if exists == 0 {
		slog.Info("Valkey kosong, menyuntikkan Seed Data fiktif")
		valkeyClient.Do(ctx, valkeyClient.B().Set().Key("metric:total_analyzed").Value("5").Build())
		valkeyClient.Do(ctx, valkeyClient.B().Set().Key("metric:total_blocked").Value("2").Build())

		mockLogs := []map[string]any{
			{"timestamp": time.Now().Format("15:04:05"), "ip": "192.168.1.10", "method": "POST", "path": "/api/login", "crime_coefficient": 95, "status": "BAHAYA", "reason": "SQL Injection detected in password field"},
			{"timestamp": time.Now().Add(-1 * time.Minute).Format("15:04:05"), "ip": "10.0.0.5", "method": "GET", "path": "/api/search", "crime_coefficient": 88, "status": "BAHAYA", "reason": "XSS payload in query parameter"},
			{"timestamp": time.Now().Add(-2 * time.Minute).Format("15:04:05"), "ip": "172.16.0.2", "method": "GET", "path": "/api/health", "crime_coefficient": 10, "status": "AMAN", "reason": "Health check endpoint"},
			{"timestamp": time.Now().Add(-3 * time.Minute).Format("15:04:05"), "ip": "45.33.32.156", "method": "POST", "path": "/api/upload", "crime_coefficient": 5, "status": "AMAN", "reason": "Normal file upload"},
			{"timestamp": time.Now().Add(-4 * time.Minute).Format("15:04:05"), "ip": "104.248.1.1", "method": "GET", "path": "/wp-admin", "crime_coefficient": 20, "status": "AMAN", "reason": "Admin access attempt"},
		}

		for _, l := range mockLogs {
			b, _ := json.Marshal(l)
			valkeyClient.Do(ctx, valkeyClient.B().Rpush().Key("recent_logs").Element(string(b)).Build())
		}
	}
}

// ========================================================================
// SYSTEM PROMPT — Instruksi Kognitif untuk Gemini
// ========================================================================

const systemPrompt = `Anda adalah mesin WAF (Web Application Firewall) Layer 7 yang beroperasi secara deterministik.
Tugas Anda: Analisis HTTP request berikut untuk mendeteksi serangan keamanan termasuk namun tidak terbatas pada:
- SQL Injection (SQLi)
- Cross-Site Scripting (XSS)
- Remote Code Execution (RCE)
- Path Traversal / LFI / RFI
- Command Injection
- Server-Side Request Forgery (SSRF)
- NoSQL Injection
- LDAP Injection
- Header Injection / CRLF Injection

Aturan penilaian Crime Coefficient (0-100):
- 0-30: Request normal, tidak ada indikasi serangan.
- 31-74: Mencurigakan tapi belum cukup bukti untuk pemblokiran.
- 75-100: Terdeteksi payload berbahaya. WAJIB BLOKIR.

Kembalikan HANYA objek JSON murni tanpa markdown, tanpa komentar, tanpa backtick.`

// ========================================================================
// HANDLER: POST /api/v1/eval
// ========================================================================

// HandlePayloadEvaluation menerima payload HTTP dari Sibyl-Proxy,
// mendelegasikan evaluasi ke Gemini API, dan mengembalikan putusan.
func HandlePayloadEvaluation(w http.ResponseWriter, r *http.Request) {
	// Proteksi heap: batasi body maksimal 2MB (TDD §1.3).
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("gagal baca body", "error", err)
		http.Error(w, `{"error":"payload terlalu besar atau tidak valid"}`, http.StatusRequestEntityTooLarge)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req EvalRequest
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Warn("JSON decode error", "error", err)
		http.Error(w, `{"error":"format JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	// Validasi field wajib.
	if req.ClientIP == "" || req.Path == "" {
		http.Error(w, `{"error":"client_ip dan path wajib diisi"}`, http.StatusBadRequest)
		return
	}

	// Prometheus: catat request masuk.
	metricRequestsTotal.Inc()

	slog.Info("eval incoming",
		"client_ip", req.ClientIP,
		"method", req.Method,
		"path", req.Path,
	)

	// --- DELEGASI KE GEMINI API (dengan pengukuran latensi) ---
	evalStart := time.Now()
	evalResp, err := evaluateWithGemini(req)
	evalDuration := time.Since(evalStart).Seconds()
	metricEvalLatency.Observe(evalDuration)

	if err != nil {
		slog.Error("gemini eval gagal",
			"client_ip", req.ClientIP,
			"path", req.Path,
			"latency_s", evalDuration,
			"error", err,
		)
		// Fail-Closed: jika AI gagal, tolak request demi keamanan (TDD §1.3).
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(EvalResponse{
			CrimeCoefficient: 100,
			Status:           "BAHAYA",
			Reason:           fmt.Sprintf("Fail-Closed: Mesin kognitif tidak merespons dalam batas waktu. Err: %v", err),
		})
		return
	}

	// Jika Crime Coefficient >= 75, masukkan IP ke global blacklist.
	if evalResp.CrimeCoefficient >= 75 {
		globalBlacklist.Store(req.ClientIP, true)
		metricBlocksTotal.Inc()

		slog.Warn("BLOKIR",
			"client_ip", req.ClientIP,
			"path", req.Path,
			"crime_coefficient", evalResp.CrimeCoefficient,
			"reason", evalResp.Reason,
			"latency_s", evalDuration,
		)

		if valkeyClient != nil {
			ctxBg := context.Background()
			_ = valkeyClient.Do(ctxBg, valkeyClient.B().Set().Key("banned_ip:"+req.ClientIP).Value("1").ExSeconds(86400).Build()).Error()
			_ = valkeyClient.Do(ctxBg, valkeyClient.B().Incr().Key("metric:total_blocked").Build()).Error()
		}
	} else {
		slog.Info("AMAN",
			"client_ip", req.ClientIP,
			"crime_coefficient", evalResp.CrimeCoefficient,
			"latency_s", evalDuration,
		)
	}

	// Simpan statistik global ke Valkey
	if valkeyClient != nil {
		ctxBg := context.Background()
		_ = valkeyClient.Do(ctxBg, valkeyClient.B().Incr().Key("metric:total_analyzed").Build()).Error()

		logEntry := map[string]any{
			"timestamp":         time.Now().Format("15:04:05"),
			"ip":                req.ClientIP,
			"method":            req.Method,
			"path":              req.Path,
			"crime_coefficient": evalResp.CrimeCoefficient,
			"status":            evalResp.Status,
			"reason":            evalResp.Reason,
		}
		logBytes, _ := json.Marshal(logEntry)
		_ = valkeyClient.Do(ctxBg, valkeyClient.B().Lpush().Key("recent_logs").Element(string(logBytes)).Build()).Error()
		_ = valkeyClient.Do(ctxBg, valkeyClient.B().Ltrim().Key("recent_logs").Start(0).Stop(49).Build()).Error()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(evalResp); err != nil {
		slog.Error("gagal encode eval response", "error", err)
	}
}

// evaluateWithGemini mengirim data request ke Gemini API untuk evaluasi kognitif.
// Context timeout 2 detik. Mengembalikan EvalResponse atau error.
func evaluateWithGemini(req EvalRequest) (*EvalResponse, error) {
	// Timeout ketat 2 detik untuk keseluruhan panggilan Gemini (TDD §1.3).
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	model := geminiClient.GenerativeModel("gemini-1.5-flash")

	// System Instruction — mengarahkan Gemini sebagai WAF kognitif.
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))

	// Paksa output JSON murni tanpa markdown wrapper.
	model.ResponseMIMEType = "application/json"

	// Enforce schema output agar Gemini tidak halusinasi struktur.
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"crime_coefficient": {
				Type:        genai.TypeInteger,
				Description: "Skor ancaman 0-100. >= 75 berarti BLOKIR.",
			},
			"status": {
				Type:        genai.TypeString,
				Enum:        []string{"AMAN", "BAHAYA"},
				Description: "Putusan: AMAN jika CC < 75, BAHAYA jika CC >= 75.",
			},
			"reason": {
				Type:        genai.TypeString,
				Description: "Penjelasan singkat mengapa request dianggap aman atau berbahaya.",
			},
		},
		Required: []string{"crime_coefficient", "status", "reason"},
	}

	// Suhu rendah untuk determinisme maksimal — WAF tidak boleh kreatif.
	model.SetTemperature(0.1)
	model.SetTopP(0.95)

	// Susun prompt user berisi data request mentah.
	headersJSON, _ := json.Marshal(req.Headers)
	userPrompt := fmt.Sprintf(`Analisis HTTP request berikut:
- Client IP: %s
- Method: %s
- Path: %s
- Headers: %s
- Body/Payload: %s

Tentukan Crime Coefficient dan berikan putusan.`, req.ClientIP, req.Method, req.Path, string(headersJSON), req.Payload)

	// Panggil Gemini API.
	resp, err := model.GenerateContent(ctx, genai.Text(userPrompt))
	if err != nil {
		return nil, fmt.Errorf("GenerateContent gagal: %w", err)
	}

	// Ekstrak teks dari response.
	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("gemini mengembalikan response kosong (0 candidates)")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini candidate tanpa content/parts")
	}

	// Ambil part pertama dan cast ke genai.Text.
	textPart, ok := candidate.Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("response part bukan teks: %T", candidate.Content.Parts[0])
	}

	// Parse JSON dari output Gemini ke struct EvalResponse.
	var evalResp EvalResponse
	if err := json.Unmarshal([]byte(textPart), &evalResp); err != nil {
		return nil, fmt.Errorf("gagal parse JSON dari Gemini: %w (raw: %s)", err, string(textPart))
	}

	// Sanitasi: pastikan status konsisten dengan skor.
	if evalResp.CrimeCoefficient >= 75 && evalResp.Status != "BAHAYA" {
		evalResp.Status = "BAHAYA"
	} else if evalResp.CrimeCoefficient < 75 && evalResp.Status != "AMAN" {
		evalResp.Status = "AMAN"
	}

	return &evalResp, nil
}

// ========================================================================
// HANDLER: GET /api/v1/blacklist
// ========================================================================

// HandleBlacklistDistribution menyajikan daftar IP berbahaya global
// untuk dikonsumsi oleh Sibyl-Proxy via CDN-Backed Polling (Opsi B).
//
// Header Cache-Control wajib di-set agar Cloudflare CDN men-cache
// response ini selama 15 detik, mengurangi hit ke Cloud Run.
func HandleBlacklistDistribution(w http.ResponseWriter, r *http.Request) {
	// Injeksi header CDN caching (TDD §1.4B / PRD §5 Opsi B).
	w.Header().Set("Cache-Control", "public, s-maxage=15, max-age=15")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Kumpulkan semua IP dari sync.Map ke slice.
	bannedIPs := make([]string, 0)
	globalBlacklist.Range(func(key, value any) bool {
		if ip, ok := key.(string); ok {
			bannedIPs = append(bannedIPs, ip)
		}
		return true
	})

	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]any{
		"banned_ips": bannedIPs,
		"count":      len(bannedIPs),
	}); err != nil {
		slog.Error("gagal encode blacklist response", "error", err)
	}
}

// ========================================================================
// HANDLER: GET /api/v1/stats
// ========================================================================

// HandleStats mengembalikan data statistik untuk dashboard.
func HandleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stats := map[string]any{
		"total_analyzed": 0,
		"total_blocked":  0,
		"recent_logs":    []map[string]any{},
	}

	if valkeyClient != nil {
		ctx := context.Background()

		analyzedResp := valkeyClient.Do(ctx, valkeyClient.B().Get().Key("metric:total_analyzed").Build())
		if analyzed, err := analyzedResp.AsInt64(); err == nil {
			stats["total_analyzed"] = analyzed
		}

		blockedResp := valkeyClient.Do(ctx, valkeyClient.B().Get().Key("metric:total_blocked").Build())
		if blocked, err := blockedResp.AsInt64(); err == nil {
			stats["total_blocked"] = blocked
		}

		logsResp := valkeyClient.Do(ctx, valkeyClient.B().Lrange().Key("recent_logs").Start(0).Stop(49).Build())
		if logsStrs, err := logsResp.AsStrSlice(); err == nil {
			var logs []map[string]any
			for _, ls := range logsStrs {
				var tl map[string]any
				if err := json.Unmarshal([]byte(ls), &tl); err == nil {
					logs = append(logs, tl)
				}
			}
			stats["recent_logs"] = logs
		}
	}

	_ = json.NewEncoder(w).Encode(stats)
}
