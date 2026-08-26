package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	store       *db.Store
	clientset   kubernetes.Interface
	registry    *k8s.ServerRegistry
	namespace   string
	trigger     chan struct{}
	mu          sync.Mutex
	driftCounts map[string]int
}

func NewController(store *db.Store, clientset kubernetes.Interface, registry *k8s.ServerRegistry, namespace string) *Controller {
	return &Controller{
		store:       store,
		clientset:   clientset,
		registry:    registry,
		namespace:   namespace,
		trigger:     make(chan struct{}, 1),
		mu:          sync.Mutex{},
		driftCounts: make(map[string]int),
	}
}

func (c *Controller) Poke() {
	select {
	default:
		c.trigger <- struct{}{}
	}
}

func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.reconcilePass(ctx)
		case <-c.trigger:
			c.reconcilePass(ctx)
		}
	}
}

func (c *Controller) reconcilePass(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Reconcile(ctx); err != nil {
		log.Printf("error occurred during reconcile: %v", err)
	}
}

func (c *Controller) Reconcile(ctx context.Context) error {
	rows, err := c.store.ListServerConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to retreive server list during reconcile: %w", err)
	}

	rowNames := make(map[string]struct{})
	for _, r := range rows {
		rowNames[r.Name] = struct{}{}
	}

	for _, r := range rows {
		if err := c.convergeRow(ctx, r); err != nil {
			log.Printf("failed to converge for server '%s': %v", r.Name, err)
		}
	}
	c.driftPass(ctx, rowNames)

	return nil
}

func (c *Controller) convergeRow(ctx context.Context, row db.ServerRecord) error {
	if row.ConsecutiveFailures >= 5 {
		return nil
	}

	var valConfig k8s.ValheimConfig
	if err := json.Unmarshal([]byte(row.ConfigJSON), &valConfig); err != nil {
		if err := c.store.IncrementFailure(ctx, row.Name); err != nil {
			return fmt.Errorf("failed to increment failures for server '%s': %w", row.Name, err)
		}
		return fmt.Errorf("attempted to unmarshal corrupt config for server '%s': %w", row.Name, err)
	}

	opts := k8s.NewServerOpts(valConfig.CoreSettings.ServerName, valConfig.CoreSettings.ServerPass, &valConfig)
	mgr := k8s.NewServerManager(c.clientset, opts)

	switch row.DesiredState {
	case "running":
		if err := mgr.CreateAll(ctx); err != nil {
			log.Printf("failed to create all resources for server '%s': %v", opts.Name, err)
			return nil
		}
		if err := c.ensureReplicas(ctx, row.Name, opts.Replicas); err != nil {
			log.Printf("failed to ensure replicas for server '%s': %v", opts.Name, err)
			return nil
		}
		if err := c.store.ResetFailures(ctx, row.Name); err != nil {
			log.Printf("failed to reset failures for server '%s': %w", err)
			return nil
		}
	case "stopped":
		if err := mgr.CreateAll(ctx); err != nil {
			log.Printf("failed to create all resources for server '%s': %v", opts.Name, err)
			return nil
		}
		if err := c.ensureReplicas(ctx, row.Name, 0); err != nil {
			log.Printf("failed to ensure replicas for server '%s': %v", opts.Name, err)
			return nil
		}
		if err := c.store.ResetFailures(ctx, row.Name); err != nil {
			log.Printf("failed to reset failures for server '%s': %w", err)
			return nil
		}
	case "deleting":
		if err := mgr.DeleteAll(ctx, row.PurgeStorage); err != nil {
			log.Printf("failed to delete resources for server '%s': %v", opts.Name, err)
			return nil
		}
		if err := c.store.DeleteServer(ctx, row.Name); err != nil {
			if errors.Is(err, db.ErrServerNotFound) {
				log.Printf("failed to delete server '%s' from database because it was not found: %v", row.Name, err)
				return nil
			}
			log.Printf("failed to delete server '%s' from database: %v", row.Name, err)
			return nil
		}
	}
	return nil
}

func (c *Controller) ensureReplicas(ctx context.Context, name string, want int32) error {
	state, exists := c.registry.Get(name)
	if !exists {
		return fmt.Errorf("failed to ensure replicas for server '%s', does not exist in registry", name)
	}
	if state.Replicas == want {
		return nil
	}
	// Not sure how to set replicas from here
	return nil
}

func (c *Controller) driftPass(ctx context.Context, rowNames map[string]struct{}) {
	for _, s := range c.registry.List() {
		if _, exists := rowNames[s.Name]; exists {
			delete(c.driftCounts, s.Name)
			continue
		}
		c.driftCounts[s.Name]++
		if c.driftCounts[s.Name] >= 3 {
			c.store.MarkDeleting(ctx, s.Name, false)
			c.driftCounts[s.Name] = 0
			log.Printf("server '%s' marked for deletion!", s.Name)
			// Feel like this isnt quite right yet
		}
	}
}
