package api

import (
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/auth"
)

func (c *ApiConfig) HandlerLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: "logout successful"})
}
