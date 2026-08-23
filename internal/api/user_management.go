package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
)

var ErrUserExists = errors.New("user already exists")

func (c *ApiConfig) HandlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	var newUser parameters
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		log.Printf("failed to decode a create user request: %v", err)
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(newUser.Username) == "" || strings.TrimSpace(newUser.Password) == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	_, err := c.Store.GetUserByUsername(r.Context(), newUser.Username)
	if err == nil {
		http.Error(w, ErrUserExists.Error(), http.StatusConflict)
		return
	}
	if !errors.Is(err, db.ErrUserNotFound) {
		log.Printf("failed to check if user exists during creation: %v", err)
		http.Error(w, "something went wrong, failed to create user", http.StatusInternalServerError)
		return
	}

	hash, err := auth.HashPassword(newUser.Password)
	if err != nil {
		log.Printf("failed to hash password for create user: %v", err)
		http.Error(w, "something went wrong, failed to create user", http.StatusInternalServerError)
		return
	}

	if err := c.Store.CreateUser(r.Context(), newUser.Username, hash, newUser.IsAdmin); err != nil {
		log.Printf("failed to create user '%s': %v", newUser.Username, err)
		http.Error(w, "something went wrong, failed to create user", http.StatusInternalServerError)
		return
	}

	resp := struct {
		Username string `json:"username"`
		IsAdmin  bool   `json:"is_admin"`
		Message  string `json:"message"`
	}{
		Username: newUser.Username,
		IsAdmin:  newUser.IsAdmin,
		Message:  fmt.Sprintf("new user '%s' successfully created. Password must be changed at login", newUser.Username),
	}
	respondWithJson(w, http.StatusCreated, resp)
}
