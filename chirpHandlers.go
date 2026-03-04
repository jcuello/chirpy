package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jcuello/chirpy/internal/auth"
	"github.com/jcuello/chirpy/internal/database"
)

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

func handleGetChirps(w http.ResponseWriter, r *http.Request) {
	authorStringId := r.URL.Query().Get("author_id")
	authorId, _ := uuid.Parse(authorStringId)

	var chirps []database.Chirp
	var err error
	if authorStringId == "" {
		chirps, err = cfg.db.GetAllChirps(r.Context())
	} else {
		chirps, err = cfg.db.GetAuthorChirps(r.Context(), uuid.NullUUID{UUID: authorId, Valid: true})
	}

	if err != nil {
		respondWithError(w, 500, "Unable to get chirps.")
		return
	}

	sortStr := r.URL.Query().Get("sort")
	if strings.ToLower(sortStr) == "desc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Time.After(chirps[j].CreatedAt.Time)
		})
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

func handleGetSingleChirp(w http.ResponseWriter, r *http.Request) {
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
