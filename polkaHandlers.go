package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/jcuello/chirpy/internal/auth"
)

func handlePolkaWebhook(w http.ResponseWriter, r *http.Request) {
	key, err := auth.GetAPIKey(r.Header)
	if key != cfg.polkaApiKey {
		respondWithError(w, 401, "unauthorized")
		return
	}

	upgradeUserEvent := UpgradeUser{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&upgradeUserEvent)

	if err != nil {
		respondWithError(w, 400, "invalid body")
		return
	}

	if upgradeUserEvent.Event != polkaUserUpgraded {
		respondWithJson(w, 204, struct{}{})
		return
	}

	if upgradeUserEvent.Event == polkaUserUpgraded {
		err = cfg.db.UpgradeToChirpyRed(r.Context(), upgradeUserEvent.Data.UserId)
		if err == sql.ErrNoRows {
			respondWithError(w, 404, "user not found")
		} else {
			respondWithJson(w, 204, struct{}{})
		}
		return
	}
}
