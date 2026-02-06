package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	serveMux := http.ServeMux{}
	server := http.Server{}
	cfg := &apiConfig{}
	urlPrefix := "/app/"
	appFileServerHandler := http.StripPrefix(urlPrefix, http.FileServer(http.Dir(".")))

	serveMux.Handle(urlPrefix, cfg.middlewareMetricsInc(appFileServerHandler))
	serveMux.HandleFunc("/healthz", func(resp http.ResponseWriter, request *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("OK"))

	})
	serveMux.HandleFunc("/metrics", cfg.viewMetrics())
	serveMux.HandleFunc("/reset", cfg.resetMetrics())

	server.Handler = &serveMux
	server.Addr = ":8080"

	server.ListenAndServe()

}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) viewMetrics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())
		w.Write([]byte(response))
	})
}

func (cfg *apiConfig) resetMetrics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		cfg.fileserverHits.Store(0)
		response := fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())
		w.Write([]byte(response))
	})
}
