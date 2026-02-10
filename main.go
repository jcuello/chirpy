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
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jcuello/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
}

type chirpPost struct {
	Body   *string `json:"body"`
	UserId string  `json:"user_id"`
}

type chirpCreated struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      *string   `json:"body"`
	UserId    string    `json:"user_id"`
}

type chirpError struct {
	Error string `json:"error"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type UserPost struct {
	Email string `json:"email"`
}

var somethingWentWrongResponse = chirpError{Error: "Something went wrong"}

var cfg apiConfig = apiConfig{}

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
	cfg.db = dbQueries
	appUrlPrefix := "/app/"
	appFileServerHandler := http.StripPrefix(appUrlPrefix, http.FileServer(http.Dir(".")))

	serveMux.Handle(appUrlPrefix, cfg.middlewareMetricsInc(appFileServerHandler))
	serveMux.HandleFunc("GET /api/healthz", func(resp http.ResponseWriter, request *http.Request) {
		resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte("OK\n"))

	})
	serveMux.HandleFunc("POST /api/chirps", handlePostChirp)
	serveMux.HandleFunc("GET /api/chirps", handleGetChirps)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", handleGetSingleChirps)
	serveMux.HandleFunc("POST /api/users", handlePostUser)

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

func handlePostChirp(w http.ResponseWriter, r *http.Request) {
	respBody := chirpPost{}
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
	userUUID, err := uuid.Parse(respBody.UserId)

	if err != nil {
		respondWithError(w, 400, "Invalid user_id")
		return
	}

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   sql.NullString{String: cleanedBody, Valid: true},
		UserID: uuid.NullUUID{UUID: userUUID, Valid: true},
	})

	if err != nil {
		respondWithError(w, 500, "Unable to create chirp.")
		return
	}

	respondWithJson(w, 201, chirpCreated{
		Id:        chirp.ID,
		CreatedAt: chirp.CreatedAt.Time,
		UpdatedAt: chirp.UpdatedAt.Time,
		Body:      &chirp.Body.String,
		UserId:    chirp.UserID.UUID.String(),
	})
}

func handlePostUser(w http.ResponseWriter, r *http.Request) {
	respBody := UserPost{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&respBody)

	if err != nil || respBody.Email == "" {
		respondWithError(w, 400, "Invalid body")
		return
	}

	dbUser, err := cfg.db.CreateUser(r.Context(), sql.NullString{String: respBody.Email, Valid: true})

	if err != nil {
		respondWithError(w, 500, "Unable to create user")
		return
	}

	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt.Time,
		UpdatedAt: dbUser.UpdatedAt.Time,
		Email:     dbUser.Email.String,
	}

	respondWithJson(w, 201, user)
}

func handleGetChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetAllChirps(r.Context())

	if err != nil {
		respondWithError(w, 500, "Unable to get chirps.")
		return
	}

	chirpsResult := []chirpCreated{}

	for _, c := range chirps {
		chirpsResult = append(chirpsResult, chirpCreated{
			Id:        c.ID,
			CreatedAt: c.CreatedAt.Time,
			UpdatedAt: c.UpdatedAt.Time,
			Body:      &c.Body.String,
			UserId:    c.UserID.UUID.String(),
		})
	}

	respondWithJson(w, 200, chirpsResult)
}

func handleGetSingleChirps(w http.ResponseWriter, r *http.Request) {
	chirpId := r.PathValue("chirpID")
	chirpUUID, err := uuid.Parse(chirpId)

	if len(chirpId) == 0 || err != nil {
		respondWithError(w, 400, "Invalid chirp ID")
		return
	}

	c, err := cfg.db.GetChirp(r.Context(), chirpUUID)
	if !c.UserID.Valid {
		respondWithError(w, 404, "Chirp not found.")
		return
	}

	chirpsResult := chirpCreated{
		Id:        c.ID,
		CreatedAt: c.CreatedAt.Time,
		UpdatedAt: c.UpdatedAt.Time,
		Body:      &c.Body.String,
		UserId:    c.UserID.UUID.String(),
	}

	respondWithJson(w, 200, chirpsResult)
}
