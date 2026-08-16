package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/google/uuid"
	"k8s.io/client-go/kubernetes"
)

type ApiConfig struct {
	Clientset *kubernetes.Clientset
	DB        *db.Store
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

	srvRecord := db.ServerRecord{
		ID:        uuid.NewString(),
		Name:      opts.Name,
		Namespace: opts.Namespace,
		NodePort:  opts.NodePort,
		Status:    "running",
	}

	record, err := c.DB.CreateServer(r.Context(), srvRecord)
	if err != nil {
		log.Printf("failed to fetch record for provision response: %v", err)
	}

	data := provisionResponse{
		ServerRecord:  record,
		ServerOptions: opts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}

func (c *ApiConfig) HandlerDeleteGameServer(w http.ResponseWriter, r *http.Request) {
	serverName := r.PathValue("server_name")
	if serverName == "" {
		http.Error(w, "server name path parameter is required", http.StatusBadRequest)
		return
	}

	record, err := c.DB.GetServerByName(r.Context(), serverName)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "server not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
		return
	}

	var req DeleteServerRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	opts := k8s.ServerOpts{
		Name:      record.Name,
		Namespace: record.Namespace,
	}

	mgr := k8s.NewServerManager(c.Clientset, opts)
	if err := mgr.DeleteAll(r.Context(), req.DeleteStorage); err != nil {
		http.Error(w, fmt.Sprintf("k8s teardown failed: %v", err), http.StatusInternalServerError)
		return
	}

	if err := c.DB.DeleteServer(r.Context(), serverName); err != nil {
		http.Error(w, fmt.Sprintf("k8s deleted but db record cleanup failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(DeleteServerResponse{
		ServerName:    serverName,
		Status:        "deleted",
		StoragePurged: req.DeleteStorage,
	}); err != nil {
		log.Printf("failed to send delete server response for server %s: %v", serverName, err)
	}
}
