package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/web"
)

type server struct {
	httpServer *http.Server
	staticFs   fs.FS
	apiCfg     *api.ApiConfig
	cancel     context.CancelFunc
	cfg        *Config
}

func newServer(port string, apiCfg *api.ApiConfig, cfg *Config, cancel context.CancelFunc) (*server, error) {
	staticFS, err := web.GetStaticFS()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: mux,
	}

	s := &server{
		httpServer: srv,
		staticFs:   staticFS,
		apiCfg:     apiCfg,
		cfg:        cfg,
		cancel:     cancel,
	}

	// Auth protected handlers
	apiMux := http.NewServeMux()

	apiMux.HandleFunc("POST /api/v1/servers/create", s.apiCfg.HandlerCreateGameServer)
	apiMux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.apiCfg.HandlerDeleteGameServer)
	apiMux.HandleFunc("GET /api/v1/servers", s.apiCfg.HandlerListGameServers)
	apiMux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.apiCfg.HandlerTransferServer)
	apiMux.HandleFunc("POST /api/v1/servers/{server_name}/start", s.apiCfg.HandlerStartGameServer)
	apiMux.HandleFunc("POST /api/v1/servers/{server_name}/stop", s.apiCfg.HandlerStopGameServer)

	apiMux.HandleFunc("POST /api/v1/auth/logout", s.apiCfg.HandlerLogout)
	apiMux.HandleFunc("POST /api/v1/auth/password", s.apiCfg.HandlerChangePassword)

	// System command handlers, root protected
	rootMux := http.NewServeMux()
	rootMux.HandleFunc("POST /api/v1/system/shutdown", s.handlerShutdown)
	rootMux.HandleFunc("POST /api/v1/system/reset", s.handlerResetDatabase)
	apiMux.Handle("/api/v1/system/", auth.RequireRoot(rootMux))

	// Admin role protected
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("POST /api/v1/admin/users/create", s.apiCfg.HandlerCreateUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/delete/{username}", s.apiCfg.HandlerDeleteUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/suspend/{username}", s.apiCfg.HandlerSuspendUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/unsuspend/{username}", s.apiCfg.HandlerUnsuspendUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/promote/{username}", s.apiCfg.HandlerPromoteUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/demote/{username}", s.apiCfg.HandlerDemoteUser)
	adminMux.HandleFunc("POST /api/v1/admin/users/reset_password", s.apiCfg.HandlerResetPassword)
	adminMux.HandleFunc("GET /api/v1/admin/users", s.apiCfg.HandlerListUsers)
	apiMux.Handle("/api/v1/admin/", auth.RequireAdmin(adminMux))

	// Public handlers
	mux.Handle("/api/v1/", s.apiCfg.Auth.AuthMiddleware(apiMux))
	mux.HandleFunc("POST /api/v1/auth/login", s.apiCfg.HandlerLogin)
	mux.HandleFunc("GET /", s.handlerIndex)
	mux.HandleFunc("GET /healthz", s.handlerReadiness)

	return s, nil
}
