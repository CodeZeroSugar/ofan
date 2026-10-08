package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

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

	if s.Name == "defaults" || s.Name == "new" {
		return errors.New("invalid server name")
	}

	matches := re.MatchString(s.Name)
	if !matches || len(s.Name) > 63 {
		return fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", s.Name)
	}

	if s.Password == "" {
		return errors.New("password is required")
	}

	if len(s.Password) < 5 {
		return fmt.Errorf("server password must be at least 5 characters")
	}

	if s.ServerOpts == nil || s.ServerOpts.Config == (k8s.ValheimConfig{}) {
		return nil
	}
	return s.ServerOpts.Config.Validate()
}

type DeleteServerRequest struct {
	DeleteStorage TolerantBool `json:"delete_storage"`
}

type TolerantBool bool

func (tb *TolerantBool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*tb = false
		return nil
	}

	var val bool
	if err := json.Unmarshal(b, &val); err == nil {
		*tb = TolerantBool(val)
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("tolerant bool: expected bool or string, got %s", b)
	}
	lower := strings.ToLower(strings.TrimSpace(s))

	switch lower {
	case "1", "t", "true", "yes", "on":
		*tb = true
		return nil
	case "0", "f", "false", "no", "off":
		*tb = false
		return nil
	default:
		return fmt.Errorf("tolerant bool: invalid value %q", s)
	}
}
