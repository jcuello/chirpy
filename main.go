package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"unicode/utf8"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

type chirp struct {
	Body string `json:"body"`
}

type chirpError struct {
	Error string `json:"error"`
}

var somethingWentWrongResponse = chirpError{Error: "Something went wrong"}

type chirpValid struct {
	Valid bool `json:"valid"`
}

func main() {
	serveMux := http.ServeMux{}
	server := http.Server{}
	cfg := &apiConfig{}
	appUrlPrefix := "/app/"
	appFileServerHandler := http.StripPrefix(appUrlPrefix, http.FileServer(http.Dir(".")))

	serveMux.Handle(appUrlPrefix, cfg.middlewareMetricsInc(appFileServerHandler))
	serveMux.HandleFunc("GET /api/healthz", func(resp http.ResponseWriter, request *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("OK\n"))

	})
	serveMux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		respBody := chirp{}
		defer r.Body.Close()

		decoder := json.NewDecoder(r.Body)
		err := decoder.Decode(&respBody)

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			respondWithError(w, 400, "Invalid body")
			return
		}

		if utf8.RuneCountInString(respBody.Body) > 140 {
			respondWithError(w, 400, "Chirp is too long")
			return
		}

		respondWithJson(w, 200, chirpValid{Valid: true})

	})

	serveMux.HandleFunc("GET /admin/metrics", cfg.viewMetrics())
	serveMux.HandleFunc("POST /admin/reset", cfg.resetMetrics())

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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", cfg.fileserverHits.Load())
		w.Write([]byte(response))
	})
}

func (cfg *apiConfig) resetMetrics() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		cfg.fileserverHits.Store(0)
		response := fmt.Sprintf("Hits: %v\n", cfg.fileserverHits.Load())
		w.Write([]byte(response))
	})
}

func respondWithError(w http.ResponseWriter, statusCode int, msg string) {
	w.WriteHeader(statusCode)
	data, err := json.Marshal(chirpError{Error: msg})
	if err != nil {
		d, _ := json.Marshal(somethingWentWrongResponse)
		fmt.Printf("%v\n", err)
		w.Write(d)
	} else {
		w.Write(data)
	}
}

func respondWithJson(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.WriteHeader(statusCode)
	data, err := json.Marshal(payload)
	if err != nil {
		d, _ := json.Marshal(somethingWentWrongResponse)
		fmt.Printf("%v\n", err)
		w.Write(d)
	} else {
		w.Write(data)
	}
}
