package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func main() {
	fmt.Println("Welcome to Ofan!")

	cfg := loadConfig()
	database, err := db.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer database.Close()

	clientset, err := k8s.NewClientSet()
	if err != nil {
		log.Fatalf("could not establish k8s clientset: %v", err)
	}

	apiCfg := &api.ApiConfig{
		Clientset: clientset,
		DB:        database,
	}

	srv, err := newServer(cfg.Port, apiCfg)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	fmt.Printf("server running at: http://localhost:%s\n", cfg.Port)
	err = srv.httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatalf("server did not shutdown properly: %v", err)
	}

	log.Println("server shutting down...")
}
