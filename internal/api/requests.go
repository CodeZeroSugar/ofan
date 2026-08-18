package api

import "github.com/CodeZeroSugar/ofan/internal/k8s"

type CreateGameServer struct {
	Name       string          `json:"name"`
	Password   string          `json:"password"`
	Default    bool            `json:"default"`
	ServerOpts *k8s.ServerOpts `json:"server_opts,omitempty"`
}

func (s *CreateGameServer) ToOpts() k8s.ServerOpts {
	if s.Default || s.ServerOpts == nil || s.ServerOpts.Config == (k8s.ValheimConfig{}) {
		return k8s.NewServerOpts(s.Name, s.Password, nil)
	}
	return k8s.NewServerOpts(s.Name, s.Password, &s.ServerOpts.Config)
}

type DeleteServerRequest struct {
	DeleteStorage bool `json:"delete_storage"`
}
