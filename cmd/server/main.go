package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func main() {
	fmt.Println("Welcome to Ofan!")

	cfg := loadConfig()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	clientset, err := k8s.NewClientSet()
	if err != nil {
		log.Fatalf("could not establish k8s clientset: %v", err)
	}

	informerMgr, err := k8s.StartInformerManager(clientset, cfg.DefaultNamespace, ctx)
	if err != nil {
		log.Fatalf("failed to start informers: %v", err)
	}

	apiCfg := &api.ApiConfig{
		Clientset:       clientset,
		InformerManager: informerMgr,
		Namespace:       cfg.DefaultNamespace,
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
