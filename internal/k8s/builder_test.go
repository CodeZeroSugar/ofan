package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
