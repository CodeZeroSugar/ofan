package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/CodeZeroSugar/ofan/web"
)

type server struct {
	httpServer *http.Server
	staticFs   fs.FS
	apiCfg     *api.ApiConfig
}

func newServer(port int) (*server, error) {
	staticFS, err := web.GetStaticFS()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	clientset, err := k8s.NewClientSet()
	if err != nil {
		return nil, fmt.Errorf("could not establish k8s clientset: %w", err)
	}

	s := &server{
		httpServer: srv,
		staticFs:   staticFS,
		apiCfg: &api.ApiConfig{
			Clientset: clientset,
		},
	}

	mux.HandleFunc("GET /", s.handlerIndex)
	mux.HandleFunc("GET /healthz", s.handlerReadiness)
	mux.HandleFunc("POST /admin/shutdown", s.handlerShutdown)
	mux.HandleFunc("POST /api/v1/create", s.apiCfg.HandlerCreateGameServer)

	return s, nil
}

func (s *server) shutdown(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("server failed to shutdown properly: %v", err)
	}
}
