package main

import (
	"context"
	_ "embed"
	"io"
	"io/fs"
	"log"
	"net/http"
)

func (s *server) handlerIndex(w http.ResponseWriter, r *http.Request) {
	indexBytes, err := fs.ReadFile(s.staticFs, "index.html")
	if err != nil {
		http.Error(w, "failed to load page", http.StatusInternalServerError)
		log.Printf("failed to load page: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(indexBytes)
}

func (s *server) handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func (s *server) handlerShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, "Ofan server shutting down...")

	go s.shutdown(context.Background())
}
