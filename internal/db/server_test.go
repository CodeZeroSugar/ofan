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

func TestGetServer_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, "admin", srv.Owner)
	assert.Equal(t, "running", srv.DesiredState)
	assert.Equal(t, 0, srv.ConsecutiveFailures)
	assert.False(t, srv.PurgeStorage)
}

func TestGetServer_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetServer(ctx, "alpha")
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestMarkDeleting(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	require.NoError(t, s.MarkDeleting(ctx, "alpha", true))

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, "deleting", srv.DesiredState)
	assert.True(t, srv.PurgeStorage)
}

func TestMarkDeleting_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.ErrorIs(t, s.MarkDeleting(ctx, "alpha", true), ErrServerNotFound)
}

func TestUpdateState_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	require.NoError(t, s.UpdateState(ctx, "alpha", "stopped"))

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, "admin", srv.Owner)
	assert.Equal(t, "stopped", srv.DesiredState)
	assert.Equal(t, 0, srv.ConsecutiveFailures)
	assert.False(t, srv.PurgeStorage)
}

func TestListServerConfigs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	require.NoError(t, s.CreateServer(ctx, "bravo", "admin", testConfigJSON("alpha")))

	cfgs, err := s.ListServerConfigs(ctx)
	require.NoError(t, err)

	assert.True(t, len(cfgs) == 2)
}

func TestIncrementFailure(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	require.NoError(t, s.IncrementFailure(ctx, "alpha"))
	require.NoError(t, s.IncrementFailure(ctx, "alpha"))

	srv, _ := s.GetServer(ctx, "alpha")
	assert.Equal(t, 2, srv.ConsecutiveFailures)
}

func TestResetFailures(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	require.NoError(t, s.IncrementFailure(ctx, "alpha"))
	require.NoError(t, s.IncrementFailure(ctx, "alpha"))

	srv, _ := s.GetServer(ctx, "alpha")
	assert.Equal(t, 2, srv.ConsecutiveFailures)

	require.NoError(t, s.ResetFailures(ctx, "alpha"))

	srv, _ = s.GetServer(ctx, "alpha")
	assert.Equal(t, 0, srv.ConsecutiveFailures)
}

func TestInsertOphanTombstone(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.InsertOrphanTombstone(ctx, "alpha", testConfigJSON("alpha")))

	tomb, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, "deleting", tomb.DesiredState)
	assert.Equal(t, "admin", tomb.Owner)
	assert.False(t, tomb.PurgeStorage)
}

func TestUpdateServerConfig(t *testing.T) {
	tests := []struct {
		name          string
		srvName       string
		cfgJSON       string
		expectedJSON  string
		expectedError error
	}{
		{
			name:          "write read no error",
			srvName:       "alpha",
			cfgJSON:       testConfigJSONFull("alpha"),
			expectedJSON:  testConfigJSONFull("alpha"),
			expectedError: nil,
		},
		{
			name:          "not found error",
			srvName:       "ghost",
			cfgJSON:       testConfigJSONFull("ghost"),
			expectedJSON:  "",
			expectedError: ErrServerNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			ctx := context.Background()

			switch tt.name {
			case "write read no error":
				require.NoError(t, s.CreateServer(ctx, tt.srvName, "admin", testConfigJSON(tt.srvName)))
				err := s.UpdateServerConfig(ctx, tt.srvName, tt.cfgJSON)
				assert.Nil(t, err)
				srv, err := s.GetServer(ctx, tt.srvName)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedJSON, srv.ConfigJSON)
			case "not found error":
				require.ErrorIs(t, s.UpdateServerConfig(ctx, tt.srvName, tt.cfgJSON), tt.expectedError)
			default:
				t.Fatalf("no matching test case for '%s'", tt.name)
			}
		})
	}
}

func testConfigJSONFull(name string) string {
	return fmt.Sprintf(`{
  "core_settings": {
    "server_name": "%s",
    "world_name": "TestWorld",
    "server_pass": "updatedpass",
    "server_port": 2456,
    "server_public": true
  },
  "access_control": {
    "admin_list_ids": "12345"
  },
  "maintenance": {
    "update_cron": "0 * * * *",
    "update_if_idle": true,
    "restart_cron": "10 5 * * *",
    "restart_if_idle": true,
    "backups": true,
    "backups_if_idle": false,
    "backups_cron": "5 * * * *",
    "backups_max_age": 7,
    "backups_max_count": 10
  },
  "mods": {
    "valheim_plus": false,
    "bepinex": false
  },
  "system_settings": {
    "time_zone": "Etc/UTC",
    "puid": 1000,
    "pgid": 1000
  }
}`, name)
}
