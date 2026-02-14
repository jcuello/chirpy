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
	"github.com/jcuello/chirpy/internal/auth"
	"github.com/jcuello/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	jwtSecret      string
}

type chirpPost struct {
	Body *string `json:"body"`
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
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
}

type UserLogin struct {
	Password     string `json:"password"`
	Email        string `json:"email"`
	RefreshToken string `json:"refresh_token"`
}

type UserPost struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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
	cfg.jwtSecret = os.Getenv("JWT_SECRET")
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
	serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", handleDeleteChirps)

	serveMux.HandleFunc("POST /api/users", handlePostUser)
	serveMux.HandleFunc("PUT /api/users", handlePutChirp)

	serveMux.HandleFunc("POST /api/login", handleLogin)
	serveMux.HandleFunc("POST /api/refresh", handleRefresh)
	serveMux.HandleFunc("POST /api/revoke", handleRevoke)

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

func respondWithInternalServerError(w http.ResponseWriter) {
	respondWithError(w, 500, "Internal Server Error")
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
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	userId, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	respBody := chirpPost{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&respBody)

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

	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   sql.NullString{String: cleanedBody, Valid: true},
		UserID: uuid.NullUUID{UUID: userId, Valid: true},
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
		UserId:    userId.String(),
	})
}

func handlePostUser(w http.ResponseWriter, r *http.Request) {
	respBody := UserPost{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&respBody)

	if err != nil || respBody.Email == "" || respBody.Password == "" {
		respondWithError(w, 400, "Invalid body")
		return
	}

	hash, err := auth.HashPassword(respBody.Password)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	dbUser, err := cfg.db.CreateUser(r.Context(),
		database.CreateUserParams{
			Email:          sql.NullString{String: respBody.Email, Valid: true},
			HashedPassword: hash,
		})

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

	if chirpId == "" || err != nil {
		respondWithError(w, 400, "Invalid chirp ID")
		return
	}

	c, err := cfg.db.GetChirp(r.Context(), chirpUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, 404, "Chirp not found.")
		} else {
			respondWithInternalServerError(w)
		}
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

func handleLogin(w http.ResponseWriter, r *http.Request) {
	userLogin := UserLogin{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decodeErr := decoder.Decode(&userLogin)
	if decodeErr != nil || userLogin.Email == "" || userLogin.Password == "" {
		respondWithError(w, 400, "Invalid body")
		return
	}

	user, err := cfg.db.GetUser(r.Context(), sql.NullString{String: userLogin.Email, Valid: true})
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	passMatch, err := auth.CheckPasswordHash(userLogin.Password, user.HashedPassword)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	if !passMatch {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	accessTokenExpiration := 60 * time.Minute
	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, accessTokenExpiration)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    uuid.NullUUID{UUID: user.ID, Valid: true},
		ExpiresAt: sql.NullTime{Time: time.Now().Add(60 * 24 * time.Hour), Valid: true},
	})
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	respondWithJson(w, 200, User{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt.Time,
		UpdatedAt:    user.UpdatedAt.Time,
		Email:        user.Email.String,
		Token:        token,
		RefreshToken: refreshToken,
	})
}

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	refreshToken, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, 401, "Unauthorized")
		} else {
			respondWithInternalServerError(w)
		}
		return
	}

	newToken, err := auth.MakeJWT(refreshToken.UserID.UUID, cfg.jwtSecret, 60*time.Minute)

	respondWithJson(w, 200, struct {
		Token string `json:"token"`
	}{
		Token: newToken,
	})
}

func handleRevoke(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	refreshToken, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, 401, "Unauthorized")
		} else {
			respondWithInternalServerError(w)
		}
		return
	}

	err = cfg.db.RevokeRefreshToken(r.Context(), refreshToken.Token)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	respondWithJson(w, 204, struct{}{})
}

func handlePutChirp(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	userId, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	defer r.Body.Close()

	body := UserPost{}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&body)
	if err != nil {
		respondWithError(w, 400, "Invalid body.")
		return
	}

	newHashedPass, err := auth.HashPassword(body.Password)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	err = cfg.db.UpdateUserPassword(r.Context(), database.UpdateUserPasswordParams{
		Email:          sql.NullString{String: body.Email, Valid: true},
		HashedPassword: newHashedPass,
	})

	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	respondWithJson(w, 200, struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	}{
		ID:    userId,
		Email: body.Email,
	})
}

func handleDeleteChirps(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	userId, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, "Unauthorized")
		return
	}

	chirpStrId := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpStrId)
	if err != nil {
		respondWithError(w, 400, "Invalid chirpID")
		return
	}

	chirp, err := cfg.db.GetChirp(r.Context(), chirpID)

	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, 404, "Chirp Not Found")

		} else {
			respondWithInternalServerError(w)
		}
		return
	}

	if chirp.UserID.UUID != userId {
		respondWithError(w, 403, "Unauthorized")
		return
	}

	err = cfg.db.DeleteChirp(r.Context(), chirp.ID)
	if err != nil {
		respondWithInternalServerError(w)
		return
	}

	respondWithJson(w, 204, struct{}{})
}
