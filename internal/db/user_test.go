package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateUserRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "bob", "hash123", true)
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.Equal(t, "bob", u.Username)
	assert.True(t, u.IsAdmin)
	assert.True(t, u.MustChangePassword)
	assert.False(t, u.IsRoot)
}

func TestCreateUser_DefaultFlags(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "carol", "hash123", false)
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "carol")
	require.NoError(t, err)
	assert.Equal(t, "carol", u.Username)
	assert.False(t, u.IsAdmin)
	assert.True(t, u.MustChangePassword)
	assert.False(t, u.IsRoot)
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "bill", "hash123", false)
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "saladin")
	assert.ErrorIs(t, err, ErrUserNotFound)
	require.Nil(t, u)
}

func TestGetUserByID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "bimothy", "hash123", false)
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "bimothy")
	require.NoError(t, err)
	assert.Equal(t, "bimothy", u.Username)
	assert.False(t, u.IsAdmin)
	assert.True(t, u.MustChangePassword)
	assert.False(t, u.IsRoot)

	u2, err := store.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "bimothy", u2.Username)
	assert.False(t, u2.IsAdmin)
	assert.True(t, u2.MustChangePassword)
	assert.False(t, u2.IsRoot)
}

func TestGetUserByID_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "thomas", "hash123", false)
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "thomas")
	require.NoError(t, err)
	assert.Equal(t, "thomas", u.Username)
	assert.False(t, u.IsAdmin)
	assert.True(t, u.MustChangePassword)
	assert.False(t, u.IsRoot)

	u2, err := store.GetUserByID(ctx, 9999)
	assert.ErrorIs(t, err, ErrUserNotFound)
	require.Nil(t, u2)
}

func TestDeleteUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "thomas", "hash123", false)
	require.NoError(t, err)

	err = store.DeleteUser(ctx, "thomas")
	require.NoError(t, err)

	u, err := store.GetUserByUsername(ctx, "thomas")
	assert.ErrorIs(t, err, ErrUserNotFound)
	require.Nil(t, u)
}

func TestDeleteUser_IsRoot(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteUser(ctx, "admin")
	assert.ErrorIs(t, err, ErrIsRoot)
}

func TestDeleteUser_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DeleteUser(ctx, "billybob")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestSuspendUnsuspendUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "barnicus", "hash123", false)
	require.NoError(t, err)

	err = store.SuspendUser(ctx, "barnicus")
	require.NoError(t, err)

	user, err := store.GetUserByUsername(ctx, "barnicus")
	require.NoError(t, err)
	assert.True(t, user.IsSuspended)

	err = store.UnsuspendUser(ctx, "barnicus")
	require.NoError(t, err)

	user1, err := store.GetUserByUsername(ctx, "barnicus")
	require.NoError(t, err)
	assert.False(t, user1.IsSuspended)
}

func TestSuspendUnsuspendUserIsRoot(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.SuspendUser(ctx, "admin")
	assert.ErrorIs(t, err, ErrIsRoot)

	err = store.UnsuspendUser(ctx, "admin")
	assert.ErrorIs(t, err, ErrIsRoot)
}

func TestSuspendUnsuspend_NotFound(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.SuspendUser(ctx, "balthazar")
	assert.ErrorIs(t, err, ErrUserNotFound)

	err = store.UnsuspendUser(ctx, "balthazar")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestPromoteDemoteUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.CreateUser(ctx, "bob", "hash123", false)
	require.NoError(t, err)
	user, err := store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	require.False(t, user.IsAdmin)

	err = store.PromoteUser(ctx, "bob")
	require.NoError(t, err)
	user, err = store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.True(t, user.IsAdmin)

	err = store.DemoteUser(ctx, "bob")
	require.NoError(t, err)
	user, err = store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.False(t, user.IsAdmin)
}

func TestDemoteUser_IsRoot(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.DemoteUser(ctx, "admin")
	assert.ErrorIs(t, err, ErrIsRoot)
}

func TestUpdatePassword(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	hash1 := "hash123"
	err := store.CreateUser(ctx, "bob", hash1, false)
	require.NoError(t, err)

	err = store.UpdatePassword(ctx, "bob", "newhash")
	require.NoError(t, err)
	u, err := store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	hash2 := u.PasswordHash

	assert.NotEqual(t, hash1, hash2)
	assert.False(t, u.MustChangePassword)
}

func TestUpdatePassword_Root(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.UpdatePassword(ctx, "admin", "newhash")
	require.NoError(t, err)
	u, err := store.GetUserByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, "newhash", u.PasswordHash)
	assert.False(t, u.MustChangePassword)
}

func TestResetPassword(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	hash1 := "hash123"
	err := store.CreateUser(ctx, "bob", hash1, false)
	require.NoError(t, err)

	err = store.UpdatePassword(ctx, "bob", "newhash")
	require.NoError(t, err)
	u, err := store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	require.False(t, u.MustChangePassword)

	err = store.ResetPassword(ctx, "bob", "temphash")
	require.NoError(t, err)
	u, err = store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	assert.True(t, u.MustChangePassword)
	assert.Equal(t, "temphash", u.PasswordHash)
}
