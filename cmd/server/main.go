package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func main() {
	fmt.Println("Welcome to Ofan!")

	cfg := loadConfig()
	if cfg.SessionSecret == "" || cfg.RootPass == "" {
		log.Fatal("ensure OFAN_SESSION_SECRET and OFAN_ROOT_PASS are configured in env")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	hash, err := auth.HashPassword(cfg.RootPass)
	if err != nil {
		log.Fatalf("failed to get hash of OFAN_ROOT_PASS: %v", err)
	}

	store, err := db.NewStore(ctx, cfg.DBPath, cfg.RootUser, hash)
	if err != nil {
		log.Fatalf("failed to initialize db store: %v", err)
	}
	authManager := auth.NewManager(store, []byte(cfg.SessionSecret))
	defer store.Close()

	clientset, err := k8s.NewClientSet(cfg.KubeConfigPath)
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
		Store:           store,
		Auth:            authManager,
	}

	srv, err := newServer(cfg.Port, apiCfg, cancel)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	fmt.Printf("server running at: http://localhost:%s\n", cfg.Port)
	go func() {
		if err := srv.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server did not shutdown properly: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("server shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("server failed to shutdown properly: %v", err)
	}
}
