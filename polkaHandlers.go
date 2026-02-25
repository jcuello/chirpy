package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

func handlePolkaWebhook(w http.ResponseWriter, r *http.Request) {
	upgradeUserEvent := UpgradeUser{}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&upgradeUserEvent)

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
