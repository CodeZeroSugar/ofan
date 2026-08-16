package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	DBPath           string
	DefaultNamespace string
	KubeConfigPath   string
	InCluster        bool
}

func loadConfig() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found in working directory or project root, using environment defaults")
	}

	return Config{
		Port:             getEnv("PORT", "8080"),
		DBPath:           getEnv("DB_PATH", "./data/ofan.db"),
		DefaultNamespace: getEnv("DEFAULT_NAMESPACE", "ofan-dev"),
		KubeConfigPath:   getEnv("KUBECONFIG_PATH", filepath.Join(os.Getenv("HOME"), "kube", "config")),
		InCluster:        getEnv("IN_CLUSTER", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
