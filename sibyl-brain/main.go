package main

import (
	"embed"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TODO: Buat file HTML statis di folder templates/ untuk Landing Page dan Dashboard.
// File akan di-embed ke binary via go:embed sehingga tidak butuh filesystem saat runtime di Cloud Run.
// Referensi tema: Terminal-Brutalism / Dominator HUD (lihat GEMINI.md §4).
//
//go:embed templates/*
var dashboardTemplates embed.FS

// ========================================================================
// PROMETHEUS METRICS — Didaftarkan sekali saat init(), thread-safe.
// ========================================================================

var (
	metricRequestsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "waf_requests_total",
		Help: "Total jumlah request yang dievaluasi oleh mesin kognitif.",
	})
	metricBlocksTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "waf_blocks_total",
		Help: "Total jumlah IP yang diblokir (Crime Coefficient >= 75).",
	})
	metricEvalLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "cognitive_eval_latency_seconds",
		Help:    "Distribusi latensi panggilan evaluasi ke Gemini API.",
		Buckets: []float64{0.1, 0.25, 0.5, 0.75, 1.0, 1.5, 2.0, 3.0, 5.0},
	})
)

func init() {
	prometheus.MustRegister(metricRequestsTotal)
	prometheus.MustRegister(metricBlocksTotal)
	prometheus.MustRegister(metricEvalLatency)
}

func main() {
	// Setup structured logging — JSON ke stdout untuk observabilitas kelas industri.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Inisialisasi koneksi Gemini API. Fail-fast jika GEMINI_API_KEY kosong.
	cleanupGemini := InitGeminiClient()
	defer cleanupGemini()

	// Inisialisasi Valkey dan injeksi Seed Data jika kosong
	InitValkeyClient()
	SeedMockData()

	r := chi.NewRouter()

	// Middleware standar — logging request dan recovery dari panic.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- Observability Endpoint ---
	// /metrics WAJIB di layer statis agar tidak ter-intercept oleh logika WAF.
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// --- API Endpoints (Kontrak Data TDD §1.4) ---
	r.Post("/api/v1/eval", HandlePayloadEvaluation)
	r.Get("/api/v1/blacklist", HandleBlacklistDistribution)
	r.Get("/api/v1/stats", HandleStats)

	// --- UI Routes (Presentasi Juri) ---
	// TODO: Implementasi handler ServeLandingPage dan ServeDashboard
	// yang membaca dari dashboardTemplates (embed.FS).
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("SIBYL-BRAIN: ONLINE"))
	})
	r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("TODO: Serve dashboard.html from embed.FS"))
	})

	slog.Info("SIBYL-BRAIN: starting", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server gagal start", "error", err)
		os.Exit(1)
	}
}

