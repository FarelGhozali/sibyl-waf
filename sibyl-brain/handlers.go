package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
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
		log.Fatal("[FATAL] GEMINI_API_KEY tidak di-set. Abort.")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("[FATAL] Gagal inisialisasi Gemini client: %v", err)
	}

	geminiClient = client
	log.Println("[INIT] Gemini client berhasil diinisialisasi.")

	return func() {
		if err := geminiClient.Close(); err != nil {
			log.Printf("[WARN] Gagal menutup Gemini client: %v", err)
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

	// --- DELEGASI KE GEMINI API ---
	evalResp, err := evaluateWithGemini(req)
	if err != nil {
		log.Printf("[EVAL] Gemini error: %v", err)
		// Fail-Closed: jika AI gagal, tolak request demi keamanan (TDD §1.3).
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(EvalResponse{
			CrimeCoefficient: 100,
			Status:           "BAHAYA",
			Reason:           fmt.Sprintf("Fail-Closed: Mesin kognitif tidak merespons dalam batas waktu. Err: %v", err),
		})
		return
	}

	// Jika Crime Coefficient >= 75, masukkan IP ke global blacklist.
	if evalResp.CrimeCoefficient >= 75 {
		globalBlacklist.Store(req.ClientIP, true)
		log.Printf("[EVAL] BLOKIR → IP=%s CC=%d Reason=%s", req.ClientIP, evalResp.CrimeCoefficient, evalResp.Reason)
	} else {
		log.Printf("[EVAL] AMAN  → IP=%s CC=%d", req.ClientIP, evalResp.CrimeCoefficient)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(evalResp); err != nil {
		log.Printf("[EVAL] Gagal encode response: %v", err)
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
		return nil, fmt.Errorf("Gemini mengembalikan response kosong (0 candidates)")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini candidate tanpa content/parts")
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
		log.Printf("[BLACKLIST] Gagal encode response: %v", err)
	}
}

