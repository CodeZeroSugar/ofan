package api

import "github.com/CodeZeroSugar/ofan/internal/k8s"

type provisionResponse struct {
	Status        string         `json:"status"`
	ServerName    string         `json:"server_name"`
	NodePort      int32          `json:"node_port"`
	ServerOptions k8s.ServerOpts `json:"server_options"`
}
