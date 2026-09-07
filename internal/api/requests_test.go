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
		modTest       bool
		bepinex       bool
		valheimPlus   bool
		serverPort    int32
		nodePort      int32
		expectedError error
	}{
		{
			name:          "valid input, nil opts",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: nil,
		},
		{
			name:          "name too long",
			serverName:    "willthisservernamebevalidimadeitreadllylongsothatiwouldnthaveanynameconflictsthereisnowaythereisacharacterlimit",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "willthisservernamebevalidimadeitreadllylongsothatiwouldnthaveanynameconflictsthereisnowaythereisacharacterlimit"),
		},
		{
			name:          "not dns compliant - special characters",
			serverName:    "!l!k3$p3c!@1characters",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "!l!k3$p3c!@1characters"),
		},
		{
			name:          "password empty",
			serverName:    "alpha",
			password:      "",
			optsIsNil:     true,
			modTest:       false,
			expectedError: errors.New("password is required"),
		},
		{
			name:          "no name",
			serverName:    "",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			expectedError: errors.New("server name is required"),
		},
		{
			name:          "not dns compliant - trailing hypen",
			serverName:    "trailinghypen-",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "trailinghypen-"),
		},
		{
			name:          "not dns compliant, capital letters",
			serverName:    "DNSCOMPLIANT",
			password:      "goodpassword",
			optsIsNil:     true,
			modTest:       false,
			expectedError: fmt.Errorf("'%s' is not DNS-1123 regex compliant (lowercase alphanumeric + hyphens, max 63 characters)", "DNSCOMPLIANT"),
		},
		{
			name:          "server port is 0",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			modTest:       false,
			serverPort:    0,
			nodePort:      30001,
			expectedError: fmt.Errorf("server_port must be in range 1-65534, got %d", 0),
		},
		{
			name:          "server port is too high",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			modTest:       false,
			serverPort:    65535,
			expectedError: fmt.Errorf("server_port must be in range 1-65534, got %d", 65535),
		},
		{
			name:          "validate mod exclusivity both on",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			modTest:       true,
			bepinex:       true,
			valheimPlus:   true,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: fmt.Errorf("cannot select BepInEx and ValheimPlus, choose one"),
		},
		{
			name:          "validate mod exclusivity bepinex on",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			modTest:       true,
			bepinex:       true,
			valheimPlus:   false,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: nil,
		},
		{
			name:          "validate mod exclusivity valheim_plus on",
			serverName:    "alpha",
			password:      "goodpassword",
			optsIsNil:     false,
			modTest:       true,
			bepinex:       false,
			valheimPlus:   true,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: nil,
		},
		{
			name:          "short password",
			serverName:    "alpha",
			password:      "gud",
			optsIsNil:     false,
			modTest:       false,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: fmt.Errorf("server password must be at least 5 characters"),
		},
		{
			name:          "short password no opts",
			serverName:    "alpha",
			password:      "gud",
			optsIsNil:     true,
			modTest:       false,
			serverPort:    2457,
			nodePort:      30001,
			expectedError: fmt.Errorf("server password must be at least 5 characters"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				if tt.modTest {
					gs.ServerOpts.Config.Mods.BepInEx = tt.bepinex
					gs.ServerOpts.Config.Mods.ValheimPlus = tt.valheimPlus
				}
				err := gs.Validate()
				if tt.expectedError == nil {
					assert.NoError(t, err)
				} else {
					assert.EqualError(t, err, tt.expectedError.Error())
				}
			}
		})
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
