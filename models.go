package main

import (
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jcuello/chirpy/internal/database"
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
