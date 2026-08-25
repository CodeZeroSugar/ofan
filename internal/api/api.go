package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"

	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type ApiConfig struct {
	Clientset       kubernetes.Interface
	InformerManager *k8s.InformerManager
	Namespace       string
	Store           *db.Store
	Auth            *auth.Manager
	Poke            func()
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

	owner := auth.UserFromContext(r.Context())
	if owner == nil {
		log.Println("could not determine server owner from context, game server not created")
		http.Error(w, "could not determine server owner, game server not created", http.StatusInternalServerError)
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

	configJson, err := json.Marshal(opts.Config)
	if err != nil {
		log.Printf("failed to marshal config into json for server '%s': %w", opts.Name, err)
		http.Error(w, "something went wrong, game server not created", http.StatusInternalServerError)
		return
	}

	if err := c.Store.CreateServer(r.Context(), opts.Name, owner.Username, string(configJson)); err != nil {
		if errors.Is(err, db.ErrServerExists) {
			http.Error(w, fmt.Sprintf("game server '%s' already exists", opts.Name), http.StatusConflict)
			return
		}
		log.Printf("failed to write new server ('%s') for owner ('%s') to db: %v", opts.Name, owner.Username, err)
		http.Error(w, "something went wrong, game server not created", http.StatusInternalServerError)
		return
	}

	c.InformerManager.Registry.Upsert(opts.Name, func(s *k8s.ServerState) {
		s.Namespace = opts.Namespace
		s.Status = "provisioning"
		s.Replicas = opts.Replicas
	})

	if c.Poke != nil {
		c.Poke()
	}

	data := provisionResponse{
		ServerName:    opts.Name,
		Status:        "provisioning",
		ServerOptions: opts,
	}

	respondWithJson(w, http.StatusAccepted, data)
}

func (c *ApiConfig) HandlerDeleteGameServer(w http.ResponseWriter, r *http.Request) {
	serverName := r.PathValue("server_name")
	if serverName == "" {
		http.Error(w, "server name path parameter is required", http.StatusBadRequest)
		return
	}

	srvRec, err := c.Store.GetServer(r.Context(), serverName)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, fmt.Sprintf("attempted to delete server '%s' but it does not exist", serverName), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("something went wrong, server '%s' not deleted", serverName), http.StatusInternalServerError)
		return
	}

	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if userCtx.Username != srvRec.Owner && !userCtx.IsAdmin {
		http.Error(w, "only server owner or admin can delete", http.StatusForbidden)
		return
	}

	var req DeleteServerRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, fmt.Sprintf("failed to decode request body: %v", err), http.StatusBadRequest)
			return
		}
	}

	if req.DeleteStorage && (userCtx.Username != srvRec.Owner && !userCtx.IsRoot) {
		if userCtx.IsAdmin {
			http.Error(w, "must transfer ownership first", http.StatusForbidden)
			return
		}
		http.Error(w, "only server owner and root user can delete persistent storage", http.StatusForbidden)
		return
	}

	if err = c.Store.MarkDeleting(r.Context(), serverName, req.DeleteStorage); err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, fmt.Sprintf("attempted to mark server '%s' for deletion, but it does not exist", serverName), http.StatusNotFound)
			return
		}
		log.Printf("failed to mark server '%s' for deletion: %v", serverName, err)
		http.Error(w, fmt.Sprintf("something went wrong, server '%s' not deleted", serverName), http.StatusInternalServerError)
		return
	}

	if _, ok := c.InformerManager.Registry.Get(serverName); ok {
		c.InformerManager.Registry.Upsert(serverName, func(s *k8s.ServerState) {
			s.Status = "deleting"
		})
	}

	if c.Poke != nil {
		c.Poke()
	}

	resp := DeleteServerResponse{
		ServerName:    serverName,
		Status:        "deleting",
		StoragePurged: req.DeleteStorage,
	}
	respondWithJson(w, http.StatusAccepted, resp)
}

func (c *ApiConfig) HandlerListGameServers(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stateList := c.InformerManager.Registry.List()
	if len(stateList) == 0 {
		respondWithJson(w, http.StatusOK, stateList)
		return
	}

	if userCtx.IsRoot || userCtx.IsAdmin {
		respondWithJson(w, http.StatusOK, stateList)
		return
	}

	srvNames, err := c.Store.ListServersByOwner(r.Context(), userCtx.Username)
	if err != nil {
		log.Printf("failed to get servers owned by '%s': %v", userCtx.Username, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	filtered := make([]*k8s.ServerState, 0)
	for _, s := range stateList {
		if slices.Contains(srvNames, s.Name) {
			filtered = append(filtered, s)
		}
	}
	respondWithJson(w, http.StatusOK, filtered)
}
