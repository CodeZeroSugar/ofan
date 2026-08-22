package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/auth"
)

func (c *ApiConfig) HandlerLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var params parameters
	err := json.NewDecoder(r.Body).Decode(&params)
	if err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	user, err := c.Store.GetUserByUsername(r.Context(), params.Username)
	if err != nil {
		log.Printf("invalid login attempt for user '%s': %v", params.Username, err)
		http.Error(w, "incorrect username or password", http.StatusUnauthorized)
		return
	}

	valid, err := auth.ValidatePassword(user.PasswordHash, params.Password)
	if err != nil {
		http.Error(w, "incorrect username or password", http.StatusUnauthorized)
		return
	}

	if !valid {
		http.Error(w, "incorrect username or password", http.StatusUnauthorized)
		return
	}

	if user.IsSuspended {
		http.Error(w, "account suspended", http.StatusForbidden)
		return
	}

	tokenString, err := c.Auth.IssueJWT(user.ID, user.Username)
	if err != nil {
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	loginResponse := struct {
		Token              string `json:"token"`
		MustChangePassword bool   `json:"must_change_password"`
	}{
		Token:              tokenString,
		MustChangePassword: user.MustChangePassword,
	}

	respondWithJson(w, http.StatusOK, loginResponse)
}
