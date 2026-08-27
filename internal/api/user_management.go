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

func rejectSelf(w http.ResponseWriter, r *http.Request, username string) bool {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		return true
	}
	return userCtx.Username == username
}

func (c *ApiConfig) HandlerDeleteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if rejectSelf(w, r, username) {
		http.Error(w, "cannot delete self", http.StatusForbidden)
		return
	}

	servers, err := c.Store.ListServersByOwner(r.Context(), username)
	if err != nil {
		log.Printf("failed to check for servers owned by '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("something went wrong, user '%s' not deleted", username), http.StatusInternalServerError)
		return
	}

	if len(servers) > 0 {
		http.Error(w, fmt.Sprintf("user '%s' owns %d server(s) (%s) - transfer or delete them first", username, len(servers), strings.Join(servers, ", ")), http.StatusConflict)
		return
	}

	err = c.Store.DeleteUser(r.Context(), username)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to delete user '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("failed to delete user '%s'", username), http.StatusInternalServerError)
		return
	default:
		respondWithJson(w, http.StatusOK, struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("user '%s' successfully deleted", username)})
	}
}

func (c *ApiConfig) HandlerSuspendUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if rejectSelf(w, r, username) {
		http.Error(w, "cannot suspend self", http.StatusForbidden)
		return
	}

	err := c.Store.SuspendUser(r.Context(), username)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to suspend user '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("failed to suspend user '%s'", username), http.StatusInternalServerError)
		return
	default:
		respondWithJson(w, http.StatusOK, struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("user '%s' successfully suspended", username)})
	}
}

func (c *ApiConfig) HandlerUnsuspendUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if rejectSelf(w, r, username) {
		http.Error(w, "cannot unsuspend self", http.StatusForbidden)
		return
	}

	err := c.Store.UnsuspendUser(r.Context(), username)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to unsuspend user '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("failed to unsuspend user '%s'", username), http.StatusInternalServerError)
		return
	default:
		respondWithJson(w, http.StatusOK, struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("user '%s' successfully unsuspended", username)})
	}
}

func (c *ApiConfig) HandlerPromoteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if rejectSelf(w, r, username) {
		http.Error(w, "cannot promote self", http.StatusForbidden)
		return
	}

	err := c.Store.PromoteUser(r.Context(), username)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to promote user '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("failed to promote user '%s'", username), http.StatusInternalServerError)
		return
	default:
		respondWithJson(w, http.StatusOK, struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("user '%s' successfully promoted", username)})
	}
}

func (c *ApiConfig) HandlerDemoteUser(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	if rejectSelf(w, r, username) {
		http.Error(w, "cannot demote self", http.StatusForbidden)
		return
	}

	err := c.Store.DemoteUser(r.Context(), username)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to demote user '%s': %v", username, err)
		http.Error(w, fmt.Sprintf("failed to demote user '%s'", username), http.StatusInternalServerError)
		return
	default:
		respondWithJson(w, http.StatusOK, struct {
			Message string `json:"message"`
		}{Message: fmt.Sprintf("user '%s' successfully demoted", username)})
	}
}

func (c *ApiConfig) HandlerResetPassword(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Username     string `json:"username"`
		TempPassword string `json:"temp_password"`
	}
	var params parameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(params.Username) == "" || strings.TrimSpace(params.TempPassword) == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	tempHash, err := auth.HashPassword(params.TempPassword)
	if err != nil {
		log.Printf("failed to hash temp password for user '%s': %v", params.Username, err)
		http.Error(w, "something went wrong, password reset failed", http.StatusInternalServerError)
		return
	}

	err = c.Store.ResetPassword(r.Context(), params.Username, tempHash)
	switch {
	case errors.Is(err, db.ErrUserNotFound):
		http.Error(w, db.ErrUserNotFound.Error(), http.StatusNotFound)
		return
	case errors.Is(err, db.ErrIsRoot):
		http.Error(w, db.ErrIsRoot.Error(), http.StatusForbidden)
		return
	case err != nil:
		log.Printf("failed to reset password for user '%s': %v", params.Username, err)
		http.Error(w, fmt.Sprintf("failed to reset password for user '%s'", params.Username), http.StatusInternalServerError)
		return
	default:
		resp := struct {
			Username string `json:"username"`
			Message  string `json:"message"`
		}{
			Username: params.Username,
			Message:  fmt.Sprintf("password for user '%s' successfully reset. Password must be changed on next login", params.Username),
		}
		respondWithJson(w, http.StatusOK, resp)
	}
}

func (c *ApiConfig) HandlerListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := c.Store.ListUsers(r.Context())
	if err != nil {
		log.Printf("failed to return list of users: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, users)
}
