package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/jcuello/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

type chirp struct {
	Body *string `json:"body"`
}

type chirpError struct {
	Error string `json:"error"`
}

var somethingWentWrongResponse = chirpError{Error: "Something went wrong"}

type chirpValid struct {
	CleanedBody string `json:"cleaned_body"`
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		fmt.Printf("Unable to connect to database %v\n", err)
		os.Exit(1)
	}

	dbQueries := database.New(db)

	serveMux := http.ServeMux{}
	server := http.Server{}
	cfg := &apiConfig{dbQueries: dbQueries}
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
		if err != nil || respBody.Body == nil {
			respondWithError(w, 400, "Invalid body")
			return
		}

		if utf8.RuneCountInString(*respBody.Body) > 140 {
			respondWithError(w, 400, "Chirp is too long")
			return
		}

		cleanedBody := cleanChirpBody(*respBody.Body)
		respondWithJson(w, 200, chirpValid{CleanedBody: cleanedBody})

	})

	serveMux.HandleFunc("GET /admin/metrics", cfg.viewMetrics())
	serveMux.HandleFunc("POST /admin/reset", cfg.resetMetrics())

	server.Handler = &serveMux
	server.Addr = ":8080"

	server.ListenAndServe()

}

func cleanChirpBody(body string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(body, " ")
	results := []string{}
	for _, word := range words {
		lowered := strings.ToLower(word)
		hasPunct := false

		for _, char := range lowered {
			if unicode.IsPunct(char) {
				hasPunct = true
				break
			}
		}

		if !hasPunct && slices.Contains(badWords, lowered) {
			results = append(results, "****")
		} else {
			results = append(results, word)
		}
	}
	return strings.Join(results, " ")
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
