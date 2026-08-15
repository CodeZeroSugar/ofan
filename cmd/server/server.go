package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/web"
)

type server struct {
	httpServer *http.Server
	staticFs   fs.FS
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

	s := &server{
		httpServer: srv,
		staticFs:   staticFS,
	}

	mux.HandleFunc("GET /", s.handlerIndex)
	mux.HandleFunc("GET /healthz", s.handlerReadiness)
	mux.HandleFunc("POST /admin/shutdown", s.handlerShutdown)

	return s, nil
}

func (s *server) shutdown(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("server failed to shutdown properly: %v", err)
	}
}
