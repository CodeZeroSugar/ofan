package api

import (
	"bytes"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/CodeZeroSugar/ofan/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplates_Message(t *testing.T) {
	tmpls, err := web.ParseTemplates()
	require.NoError(t, err)

	data := messageJson{Message: "here is some test data for the template"}

	buffer := &bytes.Buffer{}
	require.NoError(t, tmpls.ExecuteTemplate(buffer, "message", data))
	assert.Contains(t, buffer.String(), "here is some test data for the template")
}

func TestTemplates_Delete(t *testing.T) {
	tmpls, err := web.ParseTemplates()
	require.NoError(t, err)

	data := DeleteServerResponse{
		ServerName:    "alpha",
		Status:        "deleting",
		StoragePurged: true,
	}

	buffer := &bytes.Buffer{}
	require.NoError(t, tmpls.ExecuteTemplate(buffer, "delete", data))
	assert.Contains(t, buffer.String(), "alpha")
	assert.Contains(t, buffer.String(), "deleting")
	assert.Contains(t, buffer.String(), "true")
}

func TestTemplates_Create(t *testing.T) {
	tmpls, err := web.ParseTemplates()
	require.NoError(t, err)

	cfg := k8s.DefaultValheimConfig("alpha", "secret123")
	opts := k8s.NewServerOpts("alpha", "secret123", &cfg)

	data := provisionResponse{
		ServerName:    "alpha",
		Status:        "provisioning",
		ServerOptions: opts,
	}

	buffer := &bytes.Buffer{}
	require.NoError(t, tmpls.ExecuteTemplate(buffer, "create", data))
	assert.Contains(t, buffer.String(), "alpha")
	assert.Contains(t, buffer.String(), "provisioning")
	assert.Contains(t, buffer.String(), "secret123")
}

func TestTemplates_View(t *testing.T) {
	tmpls, err := web.ParseTemplates()
	require.NoError(t, err)

	data := map[string]ServerView{
		"alpha": {
			ServerState:  &k8s.ServerState{Status: "running"},
			DesiredState: "running",
			Health:       "healthy",
			Owner:        "bob",
		},
		"bravo": {
			ServerState:  &k8s.ServerState{Status: "stopped"},
			DesiredState: "running",
			Health:       "degraded",
			Owner:        "carol",
		},
	}

	buffer := &bytes.Buffer{}
	require.NoError(t, tmpls.ExecuteTemplate(buffer, "view", data))
	assert.Contains(t, buffer.String(), "alpha")
	assert.Contains(t, buffer.String(), "running")
	assert.Contains(t, buffer.String(), "healthy")
	assert.Contains(t, buffer.String(), "bob")
	assert.Contains(t, buffer.String(), "stopped")
	assert.Contains(t, buffer.String(), "degraded")
	assert.Contains(t, buffer.String(), "carol")

	data = map[string]ServerView{}
	buffer = &bytes.Buffer{}
	require.NoError(t, tmpls.ExecuteTemplate(buffer, "view", data))
	assert.Contains(t, buffer.String(), "No servers to display")
}
