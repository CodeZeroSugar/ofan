package api

import (
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

type provisionResponse struct {
	ServerRecord  *db.ServerRecord `json:"server_record"`
	ServerOptions k8s.ServerOpts   `json:"server_options"`
}

type DeleteServerResponse struct {
	ServerName    string `json:"server_name"`
	Status        string `json:"status"`
	StoragePurged bool   `json:"storage_purged"`
}
