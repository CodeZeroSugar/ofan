package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type ApiConfig struct {
	Clientset       kubernetes.Interface
	InformerManager *k8s.InformerManager
	Namespace       string
}

func (c *ApiConfig) HandlerCreateGameServer(w http.ResponseWriter, r *http.Request) {
	var req CreateGameServer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	opts := req.ToOpts()
	opts.Namespace = c.Namespace

	if _, exists := c.InformerManager.Registry.Get(opts.Name); exists {
		log.Printf("tried to create game server with name '%s' that already exists", opts.Name)
		http.Error(w, fmt.Sprintf("game server '%s' already exists", opts.Name), http.StatusConflict)
		return
	}
	// Put port in use check here
	if np := opts.NodePort; np > 0 {
		if name := c.InformerManager.Registry.PortInUse(np); name != "" {
			log.Printf("node_port %d already in use by server '%s'", np, name)
			http.Error(w, fmt.Sprintf("node_port %d already in use by server '%s'", np, name), http.StatusConflict)
			return
		}
		if name := c.InformerManager.Registry.PortInUse(np + 1); name != "" {
			log.Printf("node_port %d (query) already in use by server '%s'", np+1, name)
			http.Error(w, fmt.Sprintf("node_port %d already in use by server '%s'", np+1, name), http.StatusConflict)
			return
		}
	}

	mgr := k8s.NewServerManager(c.Clientset, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c.InformerManager.Registry.Upsert(opts.Name, func(s *k8s.ServerState) {
		s.Namespace = opts.Namespace
		s.Status = "provisioning"
		s.Replicas = opts.Replicas
	})

	if err := mgr.CreateAll(ctx); err != nil {
		log.Printf("failed to provision new server: %v", err)
		http.Error(w, fmt.Sprintf("provisioning failed: %v\n", err), http.StatusInternalServerError)
		c.InformerManager.Registry.Delete(opts.Name)
		return
	}

	data := provisionResponse{
		ServerName:    opts.Name,
		Status:        "provisioning",
		ServerOptions: opts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
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

	var req DeleteServerRequest
	if r.Body != nil {
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to decode request body: %v", err), http.StatusBadRequest)
			return
		}
	}

	ns := c.Namespace
	if state, ok := c.InformerManager.Registry.Get(serverName); ok && state.Namespace != "" {
		ns = state.Namespace
	}

	opts := k8s.ServerOpts{
		Name:      serverName,
		Namespace: ns,
	}

	mgr := k8s.NewServerManager(c.Clientset, opts)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mgr.DeleteAll(ctx, req.DeleteStorage); err != nil {
		http.Error(w, fmt.Sprintf("k8s teardown failed: %v", err), http.StatusInternalServerError)
		return
	}

	if _, ok := c.InformerManager.Registry.Get(serverName); ok {
		c.InformerManager.Registry.Upsert(opts.Name, func(s *k8s.ServerState) {
			s.Status = "deleting"
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(DeleteServerResponse{
		ServerName:    serverName,
		Status:        "deleting",
		StoragePurged: req.DeleteStorage,
	}); err != nil {
		log.Printf("failed to send delete server response for server %s: %v", serverName, err)
	}
}

func (c *ApiConfig) HandlerListGameServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c.InformerManager.Registry.List())
}
