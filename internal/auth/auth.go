package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess TokenType = "chirpy-access"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userId uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	utcNow := time.Now().UTC()
	issuedAt := jwt.NewNumericDate(utcNow)
	expiresAt := jwt.NewNumericDate(utcNow).Add(expiresIn)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  issuedAt,
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		Subject:   userId.String(),
	})

	key := []byte(tokenSecret)
	return token.SignedString(key)
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	claimsStruct := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claimsStruct, func(t *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
		id, err := uuid.Parse(claims.Subject)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid user id")
		}

		if claims.Issuer != string(TokenTypeAccess) {
			return uuid.Nil, fmt.Errorf("invalid issuer")
		}
		return id, nil
	} else {
		return uuid.Nil, fmt.Errorf("unknown claims type")
	}
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := strings.TrimSpace(headers.Get("Authorization"))

	if authHeader == "" {
		return "", errors.New("authorization header not found")
	}

	authParts := strings.Fields(authHeader)
	if len(authParts) < 2 {
		return "", errors.New("invalid authorization value")
	}

	return authParts[1], nil
}
