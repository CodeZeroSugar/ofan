package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name          string
		serverName    string
		password      string
		optsIsNil     bool
		serverPort    int32
		nodePort      int32
		expectedError error
	}{
		{
			name:          "valid input, nil opts",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     true,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: nil,
		},
		{
			name:          "name too long",
			serverName:    "willthisservernamebevalidimadeitreadllylongsothatiwouldnthaveanynameconflictsthereisnowaythereisacharacterlimit",
			password:      "goodpassword",
			optsIsNil:     true,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "willthisservernamebevalidimadeitreadllylongsothatiwouldnthaveanynameconflictsthereisnowaythereisacharacterlimit"),
		},
		{
			name:          "not dns compliant - special characters",
			serverName:    "!l!k3$p3c!@1characters",
			password:      "goodpassword",
			optsIsNil:     true,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "!l!k3$p3c!@1characters"),
		},
		{
			name:          "password empty",
			serverName:    "alpha",
			password:      "",
			optsIsNil:     true,
			expectedError: errors.New("password is required"),
		},
		{
			name:          "no name",
			serverName:    "",
			password:      "goodpassword",
			optsIsNil:     true,
			expectedError: errors.New("server name is required"),
		},
		{
			name:          "not dns compliant - trailing hypen",
			serverName:    "trailinghypen-",
			password:      "goodpassword",
			optsIsNil:     true,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "trailinghypen-"),
		},
		{
			name:          "not dns compliant, capital letters",
			serverName:    "DNSCOMPLIANT",
			password:      "goodpassword",
			optsIsNil:     true,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "DNSCOMPLIANT"),
		},
		{
			name:          "server port is 0",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			serverPort:    0,
			nodePort:      30001,
			expectedError: fmt.Errorf("server_port must be in range 1-65534, got %d", 0),
		},
		{
			name:          "server port is too high",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			serverPort:    65535,
			expectedError: fmt.Errorf("server_port must be in range 1-65534, got %d", 65535),
		},
	}

	for _, tt := range tests {
		gs := &CreateGameServer{
			Name:       tt.serverName,
			Password:   tt.password,
			ServerOpts: nil,
		}
		if tt.optsIsNil {
			err := gs.Validate()
			if tt.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError.Error())
			}
		} else {
			config := k8s.DefaultValheimConfig(tt.name, tt.password)
			opts := k8s.NewServerOpts(tt.name, tt.password, &config)
			opts.Config.CoreSettings.ServerPort = tt.serverPort
			gs.ServerOpts = &opts
			err := gs.Validate()
			if tt.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError.Error())
			}
		}
	}
}

func TestToOpts(t *testing.T) {
	// Test: nil opts input
	gs := &CreateGameServer{
		Name:       "alpha",
		Password:   "secret123",
		ServerOpts: nil,
	}
	opts := gs.ToOpts()
	assert.Equal(t, "alpha", opts.Name)
	assert.Equal(t, "alpha", opts.Config.CoreSettings.ServerName)
	assert.Equal(t, "secret123", opts.Config.CoreSettings.ServerPass)
	// Test: empty opts input
	gs = &CreateGameServer{
		Name:       "alpha",
		Password:   "secret123",
		ServerOpts: &k8s.ServerOpts{},
	}
	opts = gs.ToOpts()
	assert.Equal(t, "alpha", opts.Name)
	assert.Equal(t, "alpha", opts.Config.CoreSettings.ServerName)
	assert.Equal(t, "secret123", opts.Config.CoreSettings.ServerPass)

	// Test: config carries through
	gs = &CreateGameServer{
		Name:     "alpha",
		Password: "secret123",
		ServerOpts: &k8s.ServerOpts{
			Config: k8s.ValheimConfig{
				CoreSettings: k8s.CoreSettings{
					ServerName: "alpha",
					ServerPort: 2457,
					ServerPass: "secret123",
				},
			},
		},
	}
	opts = gs.ToOpts()
	assert.Equal(t, "alpha", opts.Name)
	assert.Equal(t, "secret123", opts.Config.CoreSettings.ServerPass)
	assert.Equal(t, "alpha", opts.Config.CoreSettings.ServerName)
	assert.Equal(t, int32(2457), opts.Config.CoreSettings.ServerPort)

	// Test: config carries through with nodePort as 0
	gs = &CreateGameServer{
		Name:     "alpha",
		Password: "secret123",
		ServerOpts: &k8s.ServerOpts{
			Config: k8s.ValheimConfig{
				CoreSettings: k8s.CoreSettings{
					ServerName: "alpha",
					ServerPort: 2457,
					ServerPass: "secret123",
				},
			},
		},
	}
	opts = gs.ToOpts()
	assert.Equal(t, "alpha", opts.Name)
	assert.Equal(t, "secret123", opts.Config.CoreSettings.ServerPass)
	assert.Equal(t, "alpha", opts.Config.CoreSettings.ServerName)
	assert.Equal(t, int32(2457), opts.Config.CoreSettings.ServerPort)
}
