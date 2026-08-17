package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/web"
)

type server struct {
	httpServer *http.Server
	staticFs   fs.FS
	apiCfg     *api.ApiConfig
}

func newServer(port string, apiCfg *api.ApiConfig) (*server, error) {
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
	}

	mux.HandleFunc("GET /", s.handlerIndex)
	mux.HandleFunc("GET /healthz", s.handlerReadiness)
	mux.HandleFunc("POST /admin/shutdown", s.handlerShutdown)
	mux.HandleFunc("POST /api/v1/servers/create", s.apiCfg.HandlerCreateGameServer)
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.apiCfg.HandlerDeleteGameServer)
	mux.HandleFunc("GET /api/v1/servers", s.apiCfg.HandlerListGameServers)

	return s, nil
}

func (s *server) shutdown(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("server failed to shutdown properly: %v", err)
	}
}
