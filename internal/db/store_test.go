package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(context.Background(), "file::memory:", "admin", "testhash")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}
