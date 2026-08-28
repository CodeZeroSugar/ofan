package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestDeploymentStatus(t *testing.T) {
	one := int32(1)
	now := metav1.Now()

	tests := []struct {
		name     string
		deploy   *appsv1.Deployment
		expected string
	}{
		{"nil replicas", &appsv1.Deployment{}, "provisioning"},
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

func TestUpsertDeployment(t *testing.T) {
	// Test: Ensure upserts are handled correcting in upsertDeployment and that statuses are updated correctly
	informerManager := InformerManager{
		Registry: NewServerRegistry(),
	}

	one := int32(1)
	create := struct {
		deploy *appsv1.Deployment
	}{
		deploy: &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: &one,
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			},
			ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "ofan-test", Labels: serverLabels("alpha")},
		},
	}
	update := struct {
		deploy *appsv1.Deployment
	}{
		deploy: &appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(0),
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      0,
				ReadyReplicas: 0,
			},
			ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "ofan-test", Labels: serverLabels("alpha")},
		},
	}

	informerManager.upsertDeployment(create.deploy)
	srv := informerManager.Registry.servers["alpha"]
	assert.Equal(t, int32(1), srv.Replicas)
	assert.Equal(t, int32(1), srv.Ready)
	assert.Equal(t, "running", srv.Status)
	assert.Equal(t, srv.Namespace, "ofan-test")

	informerManager.upsertDeployment(update.deploy)
	assert.Equal(t, int32(0), srv.Replicas)
	assert.Equal(t, int32(0), srv.Ready)
	assert.Equal(t, "stopped", srv.Status)
	assert.Equal(t, srv.Namespace, "ofan-test")
}

func TestDeleteDeployment(t *testing.T) {
	// Test: Ensure deleting deployments is successful and accessing tombstones does not cause panics
	alpha := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}}
	ghost := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "ghost"}}

	tests := []struct {
		name          string
		obj           interface{}
		seed          bool
		expectRemoved bool
	}{
		{"direct deployment", alpha, true, true},
		{"tombstone wrapping deployment", cache.DeletedFinalStateUnknown{Key: "default/alpha", Obj: alpha}, true, true},
		{
			"tombstone wrapping wrong type",
			cache.DeletedFinalStateUnknown{Key: "x", Obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "alpha"},
			}},
			true, false,
		},
		{"name not in registry", ghost, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &InformerManager{Registry: NewServerRegistry()}
			if tt.seed {
				mgr.Registry.Upsert("alpha", func(s *ServerState) {
					s.Status = "running"
				})
			}
			assert.NotPanics(t, func() { mgr.deleteDeployment(tt.obj) })

			_, ok := mgr.Registry.Get("alpha")
			if tt.expectRemoved {
				assert.False(t, ok, "alpha should be removed from registry")
			} else {
				assert.True(t, ok, "alpha should remain in registry")
			}
		})
	}
}

func TestUpsertSerivce(t *testing.T) {
	tests := []struct {
		name              string
		managed           bool
		ports             []corev1.ServicePort
		expectExists      bool
		expectedNodePort  int32
		expectedQueryPort int32
	}{
		{
			name:    "happy path",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30002},
			},
			expectExists:      true,
			expectedNodePort:  30001,
			expectedQueryPort: 30002,
		},
		{
			name:    "filtered",
			managed: false,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30002},
			},
			expectExists:      false,
			expectedNodePort:  0,
			expectedQueryPort: 0,
		},
		{
			name:    "no matching ports",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "other", NodePort: 30001},
			},
			expectExists:      true,
			expectedNodePort:  0,
			expectedQueryPort: 0,
		},
	}
	for _, tt := range tests {
		mgr := &InformerManager{Registry: NewServerRegistry()}
		labels := make(map[string]string, 0)
		if tt.managed {
			labels[LabelManagedBy] = ManagedByOfan
		}
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "alpha-service",
				Labels: labels,
			},
			Spec: corev1.ServiceSpec{
				Ports: tt.ports,
			},
		}
		mgr.upsertService(svc)
		s, ok := mgr.Registry.Get("alpha")
		if tt.expectExists {
			require.True(t, ok)
			assert.Equal(t, tt.expectedNodePort, s.NodePort)
			assert.Equal(t, tt.expectedQueryPort, s.QueryPort)
		} else {
			assert.False(t, ok)
		}

	}
}
