package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	store, err := db.NewStore(context.Background(), "file::memory:",
		"admin", "testhash")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return NewManager(store, []byte("testsecret"))
}

func TestMiddleware_NoAuthHeader(t *testing.T) {
	mgr := newTestManager(t)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := mgr.AuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_MalformedHeader(t *testing.T) {
	mgr := newTestManager(t)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := mgr.AuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Add("Authorization", "Token abc")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_GarbageTokenString(t *testing.T) {
	mgr := newTestManager(t)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	handler := mgr.AuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Add("Authorization", "asdfasdkfasdfasdjfkasdfasdfa")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_ValidToken(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.store.UpdatePassword(ctx, "admin", "updatedhash"))

	user, err := mgr.store.GetUserByUsername(ctx, "admin")
	require.NoError(t, err)
	stringToken, err := mgr.IssueJWT(user.ID, user.Username, 60*time.Minute)
	require.NoError(t, err)

	var called bool
	var gotUser *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Add("Authorization", "Bearer "+stringToken)
	rec := httptest.NewRecorder()

	handler := mgr.AuthMiddleware(next)
	handler.ServeHTTP(rec, req)

	assert.True(t, called)
	require.NotNil(t, gotUser)
	assert.Equal(t, user.Username, gotUser.Username)
}

func TestMiddleware_SuspendUserRejected(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.store.CreateUser(ctx, "bob", "hash123", false))
	require.NoError(t, mgr.store.UpdatePassword(ctx, "bob", "updatedhash"))
	require.NoError(t, mgr.store.SuspendUser(ctx, "bob"))

	user, err := mgr.store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	stringToken, err := mgr.IssueJWT(user.ID, user.Username, 60*time.Minute)

	require.NoError(t, err)
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Add("Authorization", "Bearer "+stringToken)
	rec := httptest.NewRecorder()

	handler := mgr.AuthMiddleware(next)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_DeletedUserValidToken(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	require.NoError(t, mgr.store.CreateUser(ctx, "bob", "hash123", false))
	require.NoError(t, mgr.store.UpdatePassword(ctx, "bob", "updatedhash"))

	user, err := mgr.store.GetUserByUsername(ctx, "bob")
	require.NoError(t, err)
	stringToken, err := mgr.IssueJWT(user.ID, user.Username, 60*time.Minute)
	require.NoError(t, err)

	require.NoError(t, mgr.store.DeleteUser(ctx, "bob"))

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Add("Authorization", "Bearer "+stringToken)
	rec := httptest.NewRecorder()

	handler := mgr.AuthMiddleware(next)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_GateBlocksFlaggedUser(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	user, err := mgr.store.GetUserByUsername(ctx, "admin")
	require.NoError(t, err)

	token, err := mgr.IssueJWT(user.ID, user.Username, time.Hour)
	require.NoError(t, err)

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mgr.AuthMiddleware(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, called)
}

func TestMiddleware_GateAllowsFlaggedUser(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	user, err := mgr.store.GetUserByUsername(ctx, "admin")
	require.NoError(t, err)

	token, err := mgr.IssueJWT(user.ID, user.Username, time.Hour)
	require.NoError(t, err)

	var called bool
	var userCtx *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userCtx = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/password", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	mgr.AuthMiddleware(next).ServeHTTP(rec, req)

	assert.Equal(t, user.Username, userCtx.Username)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, called)
}

func TestGuard_RequireAuthNoUser(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	rec := httptest.NewRecorder()

	RequireAuth(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, called)
}

func TestGuard_RequireAuthUserPresent(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  true,
	}

	var called bool
	var gotUser *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireAuth(next).ServeHTTP(rec, req)

	assert.True(t, called)
	require.NotNil(t, gotUser)
	assert.Equal(t, user.Username, gotUser.Username)
}

func TestGuard_RequireAdminPlainUser(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  false,
	}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireAdmin(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGuard_RequireAdminPass(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  true,
	}

	var called bool
	var gotUser *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireAdmin(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, user.Username, gotUser.Username)
}

func TestGuard_RequireAdminRootPass(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  true,
		IsRoot:   true,
	}

	var called bool
	var gotUser *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireAdmin(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, user.Username, gotUser.Username)
}

func TestGuard_RequireRootNonRoot(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  true,
		IsRoot:   false,
	}

	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireRoot(next).ServeHTTP(rec, req)

	assert.False(t, called)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGuard_RequireRootPass(t *testing.T) {
	user := &db.User{
		ID:       1,
		Username: "bob",
		IsAdmin:  true,
		IsRoot:   true,
	}

	var called bool
	var gotUser *db.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotUser = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, user))
	rec := httptest.NewRecorder()

	RequireRoot(next).ServeHTTP(rec, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, user.Username, gotUser.Username)
}
