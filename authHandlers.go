package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jcuello/chirpy/internal/auth"
	"github.com/jcuello/chirpy/internal/database"
)

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
