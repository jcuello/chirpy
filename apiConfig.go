package main

import (
	"fmt"
	"net/http"
	"os"
)

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) viewMetrics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())
		w.Write([]byte(response))
	})
}

func (cfg *apiConfig) resetMetrics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		if os.Getenv("PLATFORM") == "dev" {
			w.WriteHeader(http.StatusOK)
			cfg.fileserverHits.Store(0)
			response := fmt.Sprintf("Hits: %v\n", cfg.fileserverHits.Load())
			w.Write([]byte(response))

			err := cfg.db.DeleteAllUsers(r.Context())
			if err != nil {
				w.Write([]byte("Unable to delete users.\n"))
			} else {
				w.Write([]byte("Deleted all users.\n"))
			}
		} else {
			w.WriteHeader(403)
		}

	})
}
