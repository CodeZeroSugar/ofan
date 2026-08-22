package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/CodeZeroSugar/ofan/internal/auth"
)

func (c *ApiConfig) HandlerChangePassword(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		NewPassword string `json:"new_password"`
	}
	var params parameters
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(params.NewPassword) == "" {
		http.Error(w, "blank password not allowed", http.StatusBadRequest)
		return
	}
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	newHash, err := auth.HashPassword(params.NewPassword)
	if err != nil {
		log.Printf("failed to hash new password for user '%s': %v", user.Username, err)
		http.Error(w, "something went wrong, password not updated", http.StatusInternalServerError)
		return
	}

	err = c.Store.UpdatePassword(r.Context(), user.Username, newHash)
	if err != nil {
		log.Printf("failed to update password for user '%s': %v", user.Username, err)
		http.Error(w, "something went wrong, password not updated", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{"password updated successfully"})
}
