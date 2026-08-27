package main

import (
	_ "embed"
	"io"
	"io/fs"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/auth"
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
	s.cancel()
}

func (s *server) handlerResetDatabase(w http.ResponseWriter, r *http.Request) {
	if err := s.apiCfg.Store.ResetDatabase(r.Context()); err != nil {
		log.Printf("failed to reset database: %v", err)
		http.Error(w, "failed to reset database", http.StatusInternalServerError)
		return
	}
	hash, err := auth.HashPassword(s.cfg.RootPass)
	if err != nil {
		log.Printf("failed to hash root user password on database reset: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if err = s.apiCfg.Store.BootstrapAdmin(r.Context(), s.cfg.RootUser, hash); err != nil {
		log.Printf("failed to bootstrap root user to database after clearing tables: %v", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	s.apiCfg.Poke()
	w.WriteHeader(http.StatusOK)
}
