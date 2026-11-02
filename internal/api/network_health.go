package api

import (
	"log"
	"net/http"
)

func (c *ApiConfig) HandlerNetworkHealth(w http.ResponseWriter, r *http.Request) {
	if c.InformerManager == nil {
		log.Println("informers are not present, failed to check network health")
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	health, err := c.InformerManager.NetworkHealth()
	if err != nil {
		log.Printf("failed to perform network health check: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, health)
}
