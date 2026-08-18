package api

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

var re = regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

type CreateGameServer struct {
	Name       string          `json:"name"`
	Password   string          `json:"password"`
	Default    bool            `json:"default"`
	ServerOpts *k8s.ServerOpts `json:"server_opts,omitempty"`
}

func (s *CreateGameServer) ToOpts() k8s.ServerOpts {
	if s.Default {
		return k8s.NewServerOpts(s.Name, s.Password, nil)
	}
	return k8s.NewServerOpts(s.Name, s.Password, &s.ServerOpts.Config)
}

func (s *CreateGameServer) Validate() error {
	if s.Name == "" {
		return errors.New("server name is required")
	}
	matches := re.MatchString(s.Name)
	if !matches || len(s.Name) > 63 {
		return fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", s.Name)
	}
	if s.Password == "" {
		return errors.New("password is required")
	}

	if s.ServerOpts == nil || s.ServerOpts.Config == (k8s.ValheimConfig{}) {
		s.Default = true
		return nil
	}

	if p := s.ServerOpts.Config.CoreSettings.ServerPort; p < 1 || p > 65534 {
		return fmt.Errorf("server_port must be in range 1-65534, got %d", p)
	}

	if np := s.ServerOpts.NodePort; np != 0 && (np < 30000 || np > 32766) {
		return fmt.Errorf("node_port must be 0 (auto-assign) or in range 30000-32766, got %d", np)
	}
	return nil
}

type DeleteServerRequest struct {
	DeleteStorage bool `json:"delete_storage"`
}
