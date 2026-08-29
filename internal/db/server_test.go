package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigJSON(name string) string {
	return fmt.Sprintf(`{"core_settings":{"server_name":%q}}`, name)
}

func TestCreateServer_GetOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	require.NoError(t, err)

	owner, err := store.GetServerOwner(ctx, "alpha")
	assert.NoError(t, err)
	assert.Equal(t, "admin", owner)
}

func TestCreateServer_Duplicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	require.NoError(t, err)

	err = store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	assert.ErrorIs(t, err, ErrServerExists)
}

func TestCreateServer_BadOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateServer(ctx, "alpha", "ghost", testConfigJSON("alpha"))
	require.Error(t, err)
}

func TestDeleteServer(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	require.NoError(t, err)

	err = store.DeleteServer(ctx, "alpha")
	require.NoError(t, err)

	owner, err := store.GetServerOwner(ctx, "alpha")
	assert.ErrorIs(t, err, ErrServerNotFound)
	assert.Equal(t, "", owner)
}

func TestDeleteServer_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteServer(ctx, "ghost")
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestGetServerOwner_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	owner, err := store.GetServerOwner(ctx, "ghost")
	assert.ErrorIs(t, err, ErrServerNotFound)
	assert.Equal(t, "", owner)
}

func TestListServersByOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "bob", "hash123", false)
	require.NoError(t, err)
	err = store.CreateUser(ctx, "alice", "hash123", false)
	require.NoError(t, err)

	err = store.CreateServer(ctx, "alpha", "bob", testConfigJSON("alpha"))
	require.NoError(t, err)
	err = store.CreateServer(ctx, "bravo", "bob", testConfigJSON("bravo"))
	require.NoError(t, err)

	err = store.CreateServer(ctx, "charlie", "alice", testConfigJSON("charlie"))
	require.NoError(t, err)

	bobServers, err := store.ListServersByOwner(ctx, "bob")
	require.NoError(t, err)
	aliceServers, err := store.ListServersByOwner(ctx, "alice")
	require.NoError(t, err)
	adminServers, err := store.ListServersByOwner(ctx, "admin")
	assert.Nil(t, adminServers)
	assert.NoError(t, err)

	assert.ElementsMatch(t, []string{"alpha", "bravo"}, bobServers)
	assert.ElementsMatch(t, []string{"charlie"}, aliceServers)
}

func TestTransferServer(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	err := store.CreateUser(ctx, "bob", "hash123", false)
	require.NoError(t, err)
	err = store.CreateUser(ctx, "alice", "hash123", false)
	require.NoError(t, err)

	err = store.CreateServer(ctx, "alpha", "bob", testConfigJSON("alpha"))
	require.NoError(t, err)
	err = store.CreateServer(ctx, "bravo", "bob", testConfigJSON("bravo"))
	require.NoError(t, err)

	err = store.CreateServer(ctx, "charlie", "alice", testConfigJSON("charlie"))
	require.NoError(t, err)

	err = store.TransferServer(ctx, "bravo", "alice")
	require.NoError(t, err)
	owner, err := store.GetServerOwner(ctx, "bravo")
	require.NoError(t, err)
	assert.Equal(t, "alice", owner)
}

func TestTransferServer_BadOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	require.NoError(t, err)

	err = store.TransferServer(ctx, "alpha", "ghost")
	assert.Error(t, err)
}
