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
		Servers  map[string]ServerView
	}{
		Title:    "Servers",
		Username: userCtx.Username,
		Servers:  vm,
	}

	respondWithHTML(w, http.StatusOK, c.Templates, "servers", data)
}

func (c *ApiConfig) HandlerCreatePage(w http.ResponseWriter, r *http.Request) {
	respondWithHTML(w, http.StatusOK, c.Templates, "create", "{}")
}
