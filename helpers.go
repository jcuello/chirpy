package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"unicode"
)

var somethingWentWrongResponse = chirpError{Error: "Something went wrong"}

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
