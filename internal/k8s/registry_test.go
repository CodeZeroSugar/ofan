package k8s

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpsert(t *testing.T) {
	// Test: CreatedAt and UpdatedAt properly handled
	r := NewServerRegistry()
	first := r.Upsert("alpha", func(s *ServerState) {
		s.Status = "provisioning"
	})
	createdAt := first.CreatedAt
	updatedAt := first.UpdatedAt
	assert.NotZero(t, first.CreatedAt)
	assert.NotZero(t, first.UpdatedAt)
	time.Sleep(time.Millisecond)
	r.Upsert("alpha", func(s *ServerState) {
		s.Status = "running"
	})

	assert.Equal(t, createdAt, r.servers["alpha"].CreatedAt, "CreatedAt must not change on update")
	assert.True(t, r.servers["alpha"].UpdatedAt.After(updatedAt), "UpdatedAt must advance on update")
	assert.Equal(t, "running", r.servers["alpha"].Status)
}

func TestGet(t *testing.T) {
	// Test: Get existing and missing
	r := NewServerRegistry()
	r.Upsert("exists", func(s *ServerState) {
		s.Status = "provisioning"
	})

	_, ok := r.Get("exists")
	assert.True(t, ok)
	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestList(t *testing.T) {
	// Test: Ensure registry maintains accurate map of servers
	names := []string{"alpha", "bravo", "charlie", "delta"}
	r := NewServerRegistry()
	for _, n := range names {
		r.Upsert(n, func(s *ServerState) {
			s.Status = "running"
		})
	}
	results := r.List()
	newNames := make([]string, 0)
	for _, r := range results {
		newNames = append(newNames, r.Name)
	}
	assert.ElementsMatch(t, names, newNames, "List should return server information intact")
}

func TestDelete(t *testing.T) {
	// Test: Ensure delete is successful and does not panic if missing
	namesBefore := []string{"alpha", "bravo", "charlie", "delta"}
	r := NewServerRegistry()
	for _, n := range namesBefore {
		r.Upsert(n, func(s *ServerState) {
			s.Status = "running"
		})
	}
	namesAfter := []string{"alpha", "charlie", "delta"}
	r.Delete("bravo")
	resultAfter := make([]string, 0)
	for _, n := range r.servers {
		resultAfter = append(resultAfter, n.Name)
	}
	assert.ElementsMatch(t, namesAfter, resultAfter, "Delete should remove the specified server from registry.")
	assert.NotPanicsf(t, func() { r.Delete("echo") }, "Delete should not cause panic if specified server does not exist")
}

func TestRegistryConcurrency(t *testing.T) {
	r := NewServerRegistry()
	const workers = 32
	const iters = 100

	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			name := fmt.Sprintf("server-%d", id)
			for j := 0; j < iters; j++ {
				r.Upsert(name, func(s *ServerState) { s.Status = "running" })
				r.Get(name)
				r.List()
				if j%7 == 0 {
					r.Delete(name)
				}
			}
			r.Upsert(name, func(s *ServerState) { s.Status = "stopped" })
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		_, ok := r.Get(fmt.Sprintf("server-%d", i))
		assert.True(t, ok)
	}
	assert.Len(t, r.List(), workers)
}

func TestRegistryUpdateExisting(t *testing.T) {
	r := NewServerRegistry()

	entry := r.Upsert("alpha", func(s *ServerState) {
		s.Status = "running"
		s.NodePort = 30001
		s.QueryPort = 30002
	})

	assert.True(t, r.UpdateIfExists("alpha", func(s *ServerState) {
		s.NodePort = 0
		s.QueryPort = 0
	}))

	entry, ok := r.Get("alpha")
	assert.True(t, ok)
	assert.Equal(t, int32(0), entry.NodePort)
	assert.Equal(t, int32(0), entry.QueryPort)
}

func TestRegistryMissingEntry(t *testing.T) {
	r := NewServerRegistry()

	assert.False(t, r.UpdateIfExists("ghost", func(s *ServerState) {
		s.NodePort = 0
		s.QueryPort = 0
	}))
	_, ok := r.Get("ghost")
	assert.False(t, ok)
}
