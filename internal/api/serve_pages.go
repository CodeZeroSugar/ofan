package api

import (
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/auth"
)

func (c *ApiConfig) HandlerLoginPage(w http.ResponseWriter, r *http.Request) {
	respondWithHTML(w, http.StatusOK, c.Templates, "login", "{}")
}

func (c *ApiConfig) HandlerServersPage(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	vm, err := c.buildViewMap(r.Context(), userCtx)
	if err != nil {
		log.Printf("failed to build view map for servers page for user '%s': %v", userCtx.Username, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	data := struct {
		Title    string
		Username string
		IsAdmin  bool
		Servers  map[string]ServerView
	}{
		Title:    "Servers",
		Username: userCtx.Username,
		IsAdmin:  userCtx.IsAdmin,
		Servers:  vm,
	}

	respondWithHTML(w, http.StatusOK, c.Templates, "servers", data)
}

func (c *ApiConfig) HandlerCreatePage(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data := struct {
		IsAdmin bool
	}{
		IsAdmin: userCtx.IsAdmin,
	}
	respondWithHTML(w, http.StatusOK, c.Templates, "create", data)
}

func (c *ApiConfig) HandlerAdminUsersPage(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !userCtx.IsAdmin && !userCtx.IsRoot {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data := struct {
		IsAdmin bool
	}{
		IsAdmin: userCtx.IsAdmin,
	}

	respondWithHTML(w, http.StatusOK, c.Templates, "users", data)
}

func (c *ApiConfig) HandlerServerDetailsPage(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	data := struct {
		Name    string
		IsAdmin bool
	}{
		Name:    r.PathValue("server_name"),
		IsAdmin: userCtx.IsAdmin,
	}

	respondWithHTML(w, http.StatusOK, c.Templates, "details", data)
}
