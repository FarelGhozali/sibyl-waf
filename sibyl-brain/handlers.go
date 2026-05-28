package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
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
// HANDLER: POST /api/v1/eval
// ========================================================================

// HandlePayloadEvaluation menerima payload HTTP dari Sibyl-Proxy,
// mendelegasikan evaluasi ke Gemini API, dan mengembalikan putusan.
//
// TODO: Ganti mock response dengan integrasi Gemini SDK sesungguhnya.
func HandlePayloadEvaluation(w http.ResponseWriter, r *http.Request) {
	// Proteksi heap: batasi body maksimal 2MB (TDD §1.3).
	r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[EVAL] Gagal baca body: %v", err)
		http.Error(w, `{"error":"payload terlalu besar atau tidak valid"}`, http.StatusRequestEntityTooLarge)
		return
	}
	defer r.Body.Close()

	var req EvalRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[EVAL] JSON decode error: %v", err)
		http.Error(w, `{"error":"format JSON tidak valid"}`, http.StatusBadRequest)
		return
	}

	// Validasi field wajib.
	if req.ClientIP == "" || req.Path == "" {
		http.Error(w, `{"error":"client_ip dan path wajib diisi"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[EVAL] Incoming → IP=%s Method=%s Path=%s", req.ClientIP, req.Method, req.Path)

	// --- MOCK RESPONSE ---
	// TODO: Delegasikan ke Gemini API via google.golang.org/genai.
	// Sementara, semua request dianggap AMAN dengan skor rendah.
	resp := EvalResponse{
		CrimeCoefficient: 12,
		Status:           "AMAN",
		Reason:           "[MOCK] Evaluasi kognitif belum terhubung ke Gemini API. Default: AMAN.",
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[EVAL] Gagal encode response: %v", err)
	}
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
		log.Printf("[BLACKLIST] Gagal encode response: %v", err)
	}
}
