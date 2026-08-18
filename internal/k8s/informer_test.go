package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeploymentStatus(t *testing.T) {
	one := int32(1)
	now := metav1.Now()

	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected string
	}{
		{"nil replicas", &appsv1.Deployment{}, "stopped"},
		{"scaled to zero", &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: int32Ptr(0)}}, "stopped"},
		{"provisioning", &appsv1.Deployment{
			Spec:   appsv1.DeploymentSpec{Replicas: &one},
			Status: appsv1.DeploymentStatus{Replicas: 0},
		}, "provisioning"},
		{"starting", &appsv1.Deployment{
			Spec:   appsv1.DeploymentSpec{Replicas: &one},
			Status: appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 0},
		}, "starting"},
		{"running", &appsv1.Deployment{
			Spec:   appsv1.DeploymentSpec{Replicas: &one},
			Status: appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
		}, "running"},
		{"deleting", &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now},
			Spec:       appsv1.DeploymentSpec{Replicas: &one},
		}, "deleting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, deploymentStatus(tt.deploy))
		})
	}
}

func int32Ptr(v int32) *int32 { return &v }

func TestUpsertDeployment(t *testing.T) {}
