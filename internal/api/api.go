package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type ServerView struct {
	*k8s.ServerState
	DesiredState        string        `json:"desired_state"`
	Health              string        `json:"health"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	Uptime              time.Duration `json:"uptime"`
	Owner               string        `json:"owner,omitempty"`
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

	configJson, err := json.Marshal(opts.Config)
	if err != nil {
		log.Printf("failed to marshal config into json for server '%s': %v", opts.Name, err)
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

func (c *ApiConfig) HandlerGetGameServer(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.PathValue("server_name")

	s, err := c.Store.GetServer(r.Context(), name)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, fmt.Sprintf("server '%s' does not exist", name), http.StatusNotFound)
			return
		}
		log.Printf("failed to fetch server '%s' from database: %v", name, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if userCtx.Username != s.Owner && !userCtx.IsAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	view := &ServerView{}

	state, ok := c.InformerManager.Registry.Get(name)
	if !ok {
		view.ServerState = &k8s.ServerState{}
		view.Health = deriveHealth("", s.DesiredState, s.ConsecutiveFailures)
		view.Uptime = time.Since(s.CreatedAt)
	} else {
		view.ServerState = state
		view.Health = deriveHealth(state.Status, s.DesiredState, s.ConsecutiveFailures)
		created := s.CreatedAt
		if created.IsZero() {
			created = state.CreatedAt
		}
		view.Uptime = time.Since(created)
	}

	view.DesiredState = s.DesiredState
	view.ConsecutiveFailures = s.ConsecutiveFailures
	view.Owner = s.Owner

	respondWithJson(w, http.StatusOK, view)
}

func (c *ApiConfig) HandlerListGameServers(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stateList := c.InformerManager.Registry.List()
	if len(stateList) == 0 {
		respondWithJson(w, http.StatusOK, make(map[string]ServerView))
		return
	}

	srvRecords, err := c.Store.ListServerConfigs(r.Context())
	if err != nil {
		log.Printf("failed to get server configs for metrics: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	rowMap := make(map[string]db.ServerRecord, len(srvRecords))
	for _, r := range srvRecords {
		rowMap[r.Name] = r
	}

	viewMap := make(map[string]ServerView)
	for _, s := range stateList {
		rec := rowMap[s.Name]
		created := rec.CreatedAt
		if created.IsZero() {
			created = s.CreatedAt
		}
		viewMap[s.Name] = ServerView{
			ServerState:         s,
			DesiredState:        rec.DesiredState,
			Health:              deriveHealth(s.Status, rec.DesiredState, rec.ConsecutiveFailures),
			ConsecutiveFailures: rec.ConsecutiveFailures,
			Uptime:              time.Since(created),
			Owner:               rec.Owner,
		}
	}

	if userCtx.IsRoot || userCtx.IsAdmin {
		respondWithJson(w, http.StatusOK, viewMap)
		return
	}

	for n, s := range viewMap {
		if s.Owner != userCtx.Username {
			delete(viewMap, n)
		}
	}
	respondWithJson(w, http.StatusOK, viewMap)
}

func (c *ApiConfig) HandlerTransferServer(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	serverName := r.PathValue("server_name")
	type parameters struct {
		NewOwner string `json:"new_owner"`
	}

	var params parameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(params.NewOwner) == "" {
		http.Error(w, "new_owner is required", http.StatusBadRequest)
		return
	}

	srvRec, err := c.Store.GetServer(r.Context(), serverName)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, "server does not exist, cannot transfer ownership", http.StatusNotFound)
			return
		}
		log.Printf("failed to get server record from db for server '%s': %v", serverName, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}

	if userCtx.Username != srvRec.Owner && !userCtx.IsAdmin {
		http.Error(w, "must be server owner or admin to transfer ownership", http.StatusForbidden)
		return
	}

	if _, err := c.Store.GetUserByUsername(r.Context(), params.NewOwner); err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			http.Error(w, "user does not exist", http.StatusNotFound)
			return
		}
		log.Printf("failed to check if '%s' exists in db, server '%s' ownership not transferred", params.NewOwner, serverName)
		http.Error(w, "something went wrong, server ownership not transferred", http.StatusInternalServerError)
		return
	}

	if err := c.Store.TransferServer(r.Context(), serverName, params.NewOwner); err != nil {
		log.Printf("failed to transfer ownership of server '%s' from '%s' to '%s': %v", serverName, userCtx.Username, params.NewOwner, err)
		http.Error(w, "something went wrong, failed to transfer server ownership", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: fmt.Sprintf("successfully transferred server '%s' from '%s' to '%s'", serverName, userCtx.Username, params.NewOwner)})
}

func (c *ApiConfig) HandlerStartGameServer(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	srvName := r.PathValue("server_name")
	srvRec, err := c.Store.GetServer(r.Context(), srvName)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, db.ErrServerNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("failed to retrieve owner for server '%s': %v", srvName, err)
		http.Error(w, "something went wrong, could not start game server", http.StatusInternalServerError)
		return
	}

	if userCtx.Username != srvRec.Owner && !userCtx.IsAdmin {
		http.Error(w, fmt.Sprintf("only owner of server '%s' or admin may start", srvName), http.StatusForbidden)
		return
	}

	if srvRec.DesiredState == "running" {
		http.Error(w, fmt.Sprintf("server '%s' already running", srvName), http.StatusConflict)
		return
	}

	if err := c.Store.UpdateState(r.Context(), srvName, "running"); err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, db.ErrServerNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("failed to update state for server '%s' to 'running': %v", srvName, err)
		http.Error(w, "something went wrong, could not start game server", http.StatusInternalServerError)
		return
	}
	c.Poke()
	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: fmt.Sprintf("server '%s' successfully started", srvName)})
}

func (c *ApiConfig) HandlerStopGameServer(w http.ResponseWriter, r *http.Request) {
	userCtx := auth.UserFromContext(r.Context())
	if userCtx == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	srvName := r.PathValue("server_name")
	srvRec, err := c.Store.GetServer(r.Context(), srvName)
	if err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, db.ErrServerNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("failed to retrieve owner for server '%s': %v", srvName, err)
		http.Error(w, "something went wrong, could not stop game server", http.StatusInternalServerError)
		return
	}

	if userCtx.Username != srvRec.Owner && !userCtx.IsAdmin {
		http.Error(w, fmt.Sprintf("only owner of server '%s' or admin may stop", srvName), http.StatusForbidden)
		return
	}

	if srvRec.DesiredState == "stopped" {
		http.Error(w, fmt.Sprintf("server '%s' already stopped", srvName), http.StatusConflict)
		return
	}

	if err := c.Store.UpdateState(r.Context(), srvName, "stopped"); err != nil {
		if errors.Is(err, db.ErrServerNotFound) {
			http.Error(w, db.ErrServerNotFound.Error(), http.StatusNotFound)
			return
		}
		log.Printf("failed to update state for server '%s' to 'stopped': %v", srvName, err)
		http.Error(w, "something went wrong, could not stop game server", http.StatusInternalServerError)
		return
	}

	c.Poke()
	respondWithJson(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{Message: fmt.Sprintf("server '%s' successfully stopped", srvName)})
}

func (c *ApiConfig) HandlerDeletePVC(w http.ResponseWriter, r *http.Request) {
	srvName := r.PathValue("server_name")
	if srvName == "" {
		http.Error(w, "server name is required", http.StatusBadRequest)
		return
	}
	rec, err := c.Store.GetServer(r.Context(), srvName)
	if err != nil && !errors.Is(err, db.ErrServerNotFound) {
		log.Printf("failed to check if server '%s' exists before trying to delete PVC: %v", srvName, err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if rec != nil {
		http.Error(w, "use delete with purge_storage=true", http.StatusConflict)
		return

	}

	type parameters struct {
		Confirm bool `json:"confirm"`
	}
	var params parameters
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "invalid json payload", http.StatusBadRequest)
		return
	}

	if !params.Confirm {
		http.Error(w, "missing required confirmation", http.StatusBadRequest)
		return
	}

	if err = c.Clientset.CoreV1().PersistentVolumeClaims(c.Namespace).Delete(r.Context(), srvName+"-pvc", metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("no PVC found for server '%s'", srvName), http.StatusNotFound)
			return
		}
		log.Printf("failed to delete PVC for server '%s': %v", srvName, err)
		http.Error(w, "something went wrong, PVC not deleted", http.StatusInternalServerError)
		return
	}

	respondWithJson(w, http.StatusOK, messageJson{Message: fmt.Sprintf("PVC for server '%s' successfully deleted", srvName)})
}
