package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func main() {
	fmt.Println("Welcome to Ofan!")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := k8s.NewClientSet()
	if err != nil {
		log.Fatalf("could not create k8s clientset: %v", err)
	}
	opts := k8s.NewServerOpts("ofan-valheim", "secret123", nil)
	manager := k8s.NewServerManager(client, opts)

	if err = manager.DeleteAll(ctx, true); err != nil {
		log.Fatalf("failed to create k8s deployment: %v", err)
	}

	portStr := os.Getenv("OFAN_SERVER_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Fatalf("could not get port from env: %v", err)
	}

	srv, err := newServer(port)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	fmt.Printf("server running at: http://localhost:%d\n", port)
	err = srv.httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatalf("server did not shutdown properly: %v", err)
	}

	log.Println("server shutting down...")
}
