package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	case c.trigger <- struct{}{}:
	default:
	}
}

func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

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
			if incErr := c.store.IncrementFailure(ctx, r.Name); incErr != nil {
				log.Printf("also failed to increment failures for '%s': %v", r.Name, incErr)
			}
		}
	}
	c.driftPass(ctx, rowNames)

	return nil
}

func (c *Controller) convergeRow(ctx context.Context, row db.ServerRecord) error {
	var valConfig k8s.ValheimConfig
	if err := json.Unmarshal([]byte(row.ConfigJSON), &valConfig); err != nil {
		return fmt.Errorf("attempted to unmarshal corrupt config for server '%s': %w", row.Name, err)
	}

	srvState, exists := c.registry.Get(row.Name)
	if !exists {
		return nil
	}

	opts := k8s.NewServerOpts(valConfig.CoreSettings.ServerName, valConfig.CoreSettings.ServerPass, &valConfig)
	if srvState != nil {
		opts.NodePort = srvState.NodePort
	}
	opts.Namespace = c.namespace
	mgr := k8s.NewServerManager(c.clientset, opts)

	switch row.DesiredState {
	case "running":
		if row.ConsecutiveFailures >= 5 {
			return nil
		}
		if err := mgr.CreateAll(ctx); err != nil {
			return err
		}
		if err := c.ensureReplicas(ctx, row.Name, opts.Replicas); err != nil {
			return err
		}
		return c.store.ResetFailures(ctx, row.Name)
	case "stopped":
		if row.ConsecutiveFailures >= 5 {
			return nil
		}
		if err := mgr.CreateAll(ctx); err != nil {
			return err
		}
		if err := c.ensureReplicas(ctx, row.Name, 0); err != nil {
			return err
		}
		return c.store.ResetFailures(ctx, row.Name)
	case "deleting":
		if err := mgr.DeleteAll(ctx, row.PurgeStorage); err != nil {
			return err
		}
		return c.store.DeleteServer(ctx, row.Name)
	}
	return nil
}

func (c *Controller) ensureReplicas(ctx context.Context, name string, want int32) error {
	state, exists := c.registry.Get(name)
	if !exists || state.Replicas == want {
		return nil
	}
	dep, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment '%s': %w", name, err)
	}
	wantCopy := want
	dep.Spec.Replicas = &wantCopy
	_, err = c.clientset.AppsV1().Deployments(c.namespace).Update(ctx, dep, v1.UpdateOptions{})
	return err
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
			delete(c.driftCounts, s.Name)
			log.Printf("server '%s' marked for deletion!", s.Name)
		}
	}
}
