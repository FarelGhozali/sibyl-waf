package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// TODO: Buat file HTML statis di folder templates/ untuk Landing Page dan Dashboard.
// File akan di-embed ke binary via go:embed sehingga tidak butuh filesystem saat runtime di Cloud Run.
// Referensi tema: Terminal-Brutalism / Dominator HUD (lihat GEMINI.md §4).
//
//go:embed templates/*
var dashboardTemplates embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()

	// Middleware standar — logging request dan recovery dari panic.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// --- API Endpoints (Kontrak Data TDD §1.4) ---
	r.Post("/api/v1/eval", HandlePayloadEvaluation)
	r.Get("/api/v1/blacklist", HandleBlacklistDistribution)

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

	log.Printf("[SIBYL-BRAIN] Listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("[FATAL] Server gagal start: %v", err)
	}
}
