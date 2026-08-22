package api

import "net/http"

func (c *ApiConfig) HandlerLogout(w http.ResponseWriter, r *http.Request) {
	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: "logout successful"})
}
