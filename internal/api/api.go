package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type ApiConfig struct {
	Clientset *kubernetes.Clientset
}

func (c *ApiConfig) HandlerCreateGameServer(w http.ResponseWriter, r *http.Request) {
	var req CreateGameServer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "server name is required", http.StatusBadRequest)
		return
	}

	opts := req.ToOpts()

	mgr := k8s.NewServerManager(c.Clientset, opts)
	if err := mgr.CreateAll(r.Context()); err != nil {
		http.Error(w, fmt.Sprintf("provisioning failed: %v\n", err), http.StatusInternalServerError)
		return
	}

	data := provisionResponse{
		Status:        "provisioned",
		ServerName:    opts.Name,
		NodePort:      opts.NodePort,
		ServerOptions: opts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
