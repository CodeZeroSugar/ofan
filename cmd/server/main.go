package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
)

func main() {
	fmt.Println("Welcome to Ofan!")

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
