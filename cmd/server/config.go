package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	DefaultNamespace string
	KubeConfigPath   string
	DBPath           string
	SessionSecret    string
	RootUser         string
	RootPass         string
}

func loadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found in working directory or project root, using environment defaults")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}

	return Config{
		Port:             getEnv("PORT", "8080"),
		DefaultNamespace: getEnv("DEFAULT_NAMESPACE", "ofan-dev"),
		KubeConfigPath:   getEnv("KUBECONFIG_PATH", filepath.Join(homeDir, ".kube", "config")),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
