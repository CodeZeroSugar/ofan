package k8s

import (
	"log"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
)

type ServerState struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	NodePort  int32     `json:"node_port"`
	QueryPort int32     `json:"query_port"`
	Status    string    `json:"status"`
	Replicas  int32     `json:"replicas"`
	Ready     int32     `json:"ready_replicas"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerRegistry struct {
	mu      sync.RWMutex
	servers map[string]*ServerState
}

func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		servers: make(map[string]*ServerState),
	}
}

func (r *ServerRegistry) Get(name string) (*ServerState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.servers[name]
	if !ok {
		return nil, false
	}
	return state, true
}

func (r *ServerRegistry) List() []*ServerState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	states := make([]*ServerState, 0)
	for _, state := range r.servers {
		states = append(states, state)
	}
	return states
}

func (r *ServerRegistry) Upsert(name string, fn func(*ServerState)) *ServerState {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.servers[name]
	if !ok {
		state = &ServerState{Name: name, CreatedAt: time.Now()}
		r.servers[name] = state
	}
	state.UpdatedAt = time.Now()
	fn(state)
	return state
}

func (r *ServerRegistry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.servers[name]; !ok {
		log.Printf("ServerState for '%s' does not exist, delete failed", name)
		return
	}
	delete(r.servers, name)
}

func (r *ServerRegistry) PortInUse(port int32) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for name, s := range r.servers {
		if s.NodePort == port || s.QueryPort == port {
			return name
		}
	}
	return ""
}

func deploymentStatus(d *appsv1.Deployment) string {
	if d.DeletionTimestamp != nil {
		return "deleting"
	}
	replicas := int32(1)
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	switch {
	case replicas == 0:
		return "stopped"
	case d.Status.ReadyReplicas >= replicas && d.Status.Replicas > 0:
		return "running"
	case d.Status.Replicas == 0:
		return "provisioning"
	default:
		return "starting"
	}
}
