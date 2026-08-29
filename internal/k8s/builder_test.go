package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestBuildDeployment(t *testing.T) {
	mgr := NewServerManager(nil, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-test",
		Replicas:  int32(2),
	})

	name := mgr.opts.Name
	labels := serverLabels(name)
	dep := mgr.BuildDeployment()
	assert.Equal(t, "alpha", dep.Name)
	assert.Equal(t, "ofan-test", dep.Namespace)
	assert.Equal(t, int32(2), *dep.Spec.Replicas)
	assert.Equal(t, labels, dep.Labels)
	assert.Equal(t, labels, dep.Spec.Selector.MatchLabels)
	assert.Equal(t, labels, dep.Spec.Template.Labels)
	for _, c := range dep.Spec.Template.Spec.Containers {
		assert.Equal(t, "valheim-server", c.Name)
		assert.Equal(t, "ghcr.io/lloesche/valheim-server:latest", c.Image)
		for _, m := range c.VolumeMounts {
			assert.Equal(t, name+"-volume", m.Name)
		}
		for _, e := range c.EnvFrom {
			if e.ConfigMapRef != nil {
				assert.Equal(t, name+"-configmap", e.ConfigMapRef.Name)
			}
			if e.SecretRef != nil {
				assert.Equal(t, name+"-secret", e.SecretRef.Name)
			}
		}
	}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		assert.Equal(t, name+"-volume", v.Name)
		assert.Equal(t, name+"-pvc", v.PersistentVolumeClaim.ClaimName)
	}
}

func TestBuildService(t *testing.T) {
	tests := []struct {
		name                string
		inputServerPort     int32
		expectedQueryPort   int32
		expectedServerPort  int32
		expectedServerQuery int32
	}{
		{
			name:                "node port is 0",
			inputServerPort:     2456,
			expectedQueryPort:   0,
			expectedServerPort:  2456,
			expectedServerQuery: 2457,
		},
	}

	for _, tt := range tests {
		mgr := NewServerManager(nil, ServerOpts{
			Name:      "alpha",
			Namespace: "ofan-test",
			Config: ValheimConfig{
				CoreSettings: CoreSettings{
					ServerPort: tt.inputServerPort,
				},
				AccessControl:  AccessControl{},
				Maintenance:    Maintenance{},
				Mods:           Mods{},
				SystemSettings: SystemSettings{},
			},
		})
		svc := mgr.BuildService()
		assert.True(t, len(svc.Spec.Ports) == 2)
		sawUdp := false
		sawQuery := false
		for _, p := range svc.Spec.Ports {
			switch p.Name {
			case "valheim-udp":
				sawUdp = true
				assert.Equal(t, tt.expectedServerPort, p.Port)
				assert.Equal(t, intstr.FromInt32(tt.expectedServerPort), p.TargetPort)
				assert.Equal(t, v1.ProtocolUDP, p.Protocol)
			case "valheim-query":
				sawQuery = true
				assert.Equal(t, tt.expectedServerQuery, p.Port)
				assert.Equal(t, intstr.FromInt32(tt.expectedServerQuery), p.TargetPort)
				assert.Equal(t, v1.ProtocolUDP, p.Protocol)
			}
		}
		assert.True(t, sawUdp)
		assert.True(t, sawQuery)
		assert.Equal(t, map[string]string{"app": mgr.opts.Name}, svc.Spec.Selector)
		assert.Equal(t, serverLabels("alpha"), svc.Labels)
	}
}
