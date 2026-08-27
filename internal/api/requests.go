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
	ServerOpts *k8s.ServerOpts `json:"server_opts,omitempty"`
}

func (s *CreateGameServer) ToOpts() k8s.ServerOpts {
	var config *k8s.ValheimConfig
	if s.ServerOpts != nil && s.ServerOpts.Config != (k8s.ValheimConfig{}) {
		config = &s.ServerOpts.Config
	}
	opts := k8s.NewServerOpts(s.Name, s.Password, config)
	return opts
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
		return nil
	}

	if p := s.ServerOpts.Config.CoreSettings.ServerPort; p < 1 || p > 65534 {
		return fmt.Errorf("server_port must be in range 1-65534, got %d", p)
	}

	return nil
}

type DeleteServerRequest struct {
	DeleteStorage bool `json:"delete_storage"`
}
