package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

func TestUpsertPodManaged(t *testing.T) {
	tests := []struct {
		name string
		obj  *corev1.Pod
	}{
		{
			name: "upsert managed pod",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: serverLabels("alpha"),
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.0.2",
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "alpha",
							RestartCount: 7,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "upsert pod foreign",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{LabelManagedBy: "not-ofan"},
				},
			},
		},
		{
			name: "present not waiting",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: serverLabels("alpha"),
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.0.2",
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "alpha",
						},
					},
				},
			},
		},
		{
			name: "empty status",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: serverLabels("alpha"),
				},
				Status: corev1.PodStatus{
					HostIP:            "192.168.0.2",
					ContainerStatuses: []corev1.ContainerStatus{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := InformerManager{Registry: NewServerRegistry()}
			switch tt.name {
			case "upsert managed pod":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.upsertPod(tt.obj)
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "192.168.0.2", srv.NodeIP)
				assert.Equal(t, int32(7), srv.RestartCount)
				assert.Equal(t, "CrashLoopBackOff", srv.PodWaiting)
			case "upsert pod foreign":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.upsertPod(tt.obj)
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "", srv.NodeIP)
				assert.Equal(t, int32(0), srv.RestartCount)
				assert.Equal(t, "", srv.PodWaiting)
			case "present not waiting":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.upsertPod(tt.obj)
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "192.168.0.2", srv.NodeIP)
				assert.Equal(t, int32(0), srv.RestartCount)
				assert.Equal(t, "", srv.PodWaiting)
			case "empty status":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.upsertPod(tt.obj)
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "192.168.0.2", srv.NodeIP)
				assert.Equal(t, int32(0), srv.RestartCount)
				assert.Equal(t, "", srv.PodWaiting)
			default:
				t.Fatalf("case '%s' not recognized", tt.name)
			}
		})
	}
}

func TestDeletePod(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "managed delete resets",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: serverLabels("alpha"),
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.0.2",
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "alpha",
							RestartCount: 7,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "foreign delete ignore",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{LabelManagedBy: "not-ofan"},
				},
				Status: corev1.PodStatus{
					HostIP: "192.168.0.2",
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "alpha",
							RestartCount: 7,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "tombstone wrapped pod",
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/alpha",
				Obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "alpha",
						Labels: map[string]string{
							LabelManagedBy:  ManagedByOfan,
							LabelServerName: "alpha",
						},
					},
				},
			},
		},
		{
			name: "tombstone wrong type",
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/alpha",
				Obj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "alpha",
						Labels: map[string]string{
							LabelManagedBy:  ManagedByOfan,
							LabelServerName: "alpha",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := InformerManager{Registry: NewServerRegistry()}
			switch tt.name {
			case "managed delete resets":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.Registry.servers["alpha"].PodWaiting = "CrashLoopBackOff"
				mgr.Registry.servers["alpha"].RestartCount = 5
				mgr.Registry.servers["alpha"].NodeIP = "192.168.0.2"
				mgr.deletePod(tt.obj)
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "", srv.NodeIP)
				assert.Equal(t, int32(0), srv.RestartCount)
				assert.Equal(t, "", srv.PodWaiting)
			case "foreign delete ignore":
				mgr.deletePod(tt.obj)
				assert.True(t, len(mgr.Registry.servers) == 0)
			case "tombstone wrapped pod":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.Registry.servers["alpha"].PodWaiting = "CrashLoopBackOff"
				mgr.Registry.servers["alpha"].RestartCount = 5
				mgr.Registry.servers["alpha"].NodeIP = "192.168.0.2"
				assert.NotPanics(t, func() { mgr.deletePod(tt.obj) })
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "", srv.NodeIP)
				assert.Equal(t, int32(0), srv.RestartCount)
				assert.Equal(t, "", srv.PodWaiting)
			case "tombstone wrong type":
				mgr.Registry.servers["alpha"] = &ServerState{}
				mgr.Registry.servers["alpha"].PodWaiting = "CrashLoopBackOff"
				mgr.Registry.servers["alpha"].RestartCount = 5
				mgr.Registry.servers["alpha"].NodeIP = "192.168.0.2"
				assert.NotPanics(t, func() { mgr.deletePod(tt.obj) })
				srv := mgr.Registry.servers["alpha"]
				assert.Equal(t, "192.168.0.2", srv.NodeIP)
				assert.Equal(t, int32(5), srv.RestartCount)
				assert.Equal(t, "CrashLoopBackOff", srv.PodWaiting)
			default:
				t.Fatalf("case '%s' not recognized", tt.name)
			}
		})
	}
}

func TestDeleteService(t *testing.T) {
	tests := []struct {
		name              string
		managed           bool
		ports             []corev1.ServicePort
		expectExists      bool
		expectedNodePort  int32
		expectedQueryPort int32
		seed              bool
		obj               interface{}
	}{
		{
			name:    "entry exists",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30002},
			},
			expectExists:      true,
			expectedNodePort:  0,
			expectedQueryPort: 0,
			seed:              true,
			obj:               &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alpha-service", Labels: map[string]string{LabelManagedBy: ManagedByOfan}}},
		},
		{
			name:    "no entry",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30002},
			},
			expectExists:      false,
			expectedNodePort:  0,
			expectedQueryPort: 0,
			seed:              false,
			obj:               &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alpha-service", Labels: map[string]string{LabelManagedBy: ManagedByOfan}}},
		},
		{
			name:    "foreign delete ignored",
			managed: false,
			ports: []corev1.ServicePort{
				{Name: "other", NodePort: 30001},
				{Name: "other-one", NodePort: 30001},
			},
			expectExists:      false,
			expectedNodePort:  0,
			expectedQueryPort: 0,
			seed:              true,
			obj:               &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "alpha-service", Namespace: "not-ofan", Labels: map[string]string{LabelManagedBy: "not-by-ofan"}}},
		},
		{
			name:    "tombstone-wrapped service",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30001},
			},
			expectExists:      true,
			expectedNodePort:  0,
			expectedQueryPort: 0,
			seed:              true,
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/alpha-service",
				Obj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "alpha-service",
						Labels: map[string]string{
							LabelManagedBy: ManagedByOfan,
						},
					},
				},
			},
		},
		{
			name:    "tombstone wrapping non-service",
			managed: true,
			ports: []corev1.ServicePort{
				{Name: "valheim-udp", NodePort: 30001},
				{Name: "valheim-query", NodePort: 30002},
			},
			expectExists:      true,
			expectedNodePort:  30001,
			expectedQueryPort: 30002,
			seed:              true,
			obj: cache.DeletedFinalStateUnknown{
				Key: "default/alpha-pod",
				Obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "alpha-pod",
						Labels: map[string]string{
							LabelManagedBy: ManagedByOfan,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &InformerManager{Registry: NewServerRegistry()}
			if tt.seed {
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
			}
			assert.NotPanics(t, func() { mgr.deleteService(tt.obj) })

			s, ok := mgr.Registry.Get("alpha")
			if tt.expectExists {
				require.True(t, ok)
				assert.Equal(t, tt.expectedNodePort, s.NodePort)
				assert.Equal(t, tt.expectedQueryPort, s.QueryPort)
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestNodeInformerNotSynced(t *testing.T) {
	m := &InformerManager{nodeWatchError: errors.New("forbidden")}

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "unknown", health.State)
	assert.Equal(t, health.NodeWatchError.Error(), "forbidden")
}

func TestEndpointInformerNotSynced(t *testing.T) {
	m := &InformerManager{endpointWatchError: errors.New("forbidden")}

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "unknown", health.State)
	assert.Equal(t, health.EndpointWatchError.Error(), "forbidden")
}

func TestNilNodeLister(t *testing.T) {
	m := &InformerManager{}
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	m.NodeLister = nil

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "unknown", health.State)
}

func TestNoNodes(t *testing.T) {
	m := &InformerManager{}
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "unknown", health.State)
}

func TestNodesWithoutInternalIP(t *testing.T) {
	m := &InformerManager{}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}}}}
	client := fake.NewSimpleClientset(node)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "unknown", health.State)
}

func TestNodesMatchingIP(t *testing.T) {
	m := &InformerManager{}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.69"}}}}
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default", Labels: map[string]string{"kubernetes.io/service-name": "kubernetes"}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node, slice)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "ok", health.State)

	assert.Equal(t, []string{"192.168.20.69"}, health.NodeIPs)
	assert.Equal(t, []string{"192.168.20.69"}, health.EndpointIPs)
}

func TestNodesStaleIP(t *testing.T) {
	m := &InformerManager{}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.72"}}}}
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default", Labels: map[string]string{"kubernetes.io/service-name": "kubernetes"}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node, slice)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "drift", health.State)

	assert.Equal(t, []string{"192.168.20.72"}, health.NodeIPs)
	assert.Equal(t, []string{"192.168.20.69"}, health.EndpointIPs)
}

func TestNodesMultiple(t *testing.T) {
	m := &InformerManager{}

	node1 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.69"}}}}
	node2 := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.75"}}}}
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default", Labels: map[string]string{"kubernetes.io/service-name": "kubernetes"}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node1, node2, slice)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "ok", health.State)

	assert.ElementsMatch(t, []string{"192.168.20.69", "192.168.20.75"}, health.NodeIPs)
	assert.Equal(t, []string{"192.168.20.69"}, health.EndpointIPs)
}

func TestNodesNoEndpoints(t *testing.T) {
	m := &InformerManager{}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "drift", health.State)

	assert.Equal(t, []string{"192.168.20.69"}, health.NodeIPs)
}

func TestNodesUnrelatedSlice(t *testing.T) {
	m := &InformerManager{}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.69"}}}}
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default", Labels: map[string]string{"kubernetes.io/service-name": "differentservice"}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node, slice)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "drift", health.State)

	assert.Equal(t, []string{"192.168.20.69"}, health.NodeIPs)
}

func TestWatchErrorSynced(t *testing.T) {
	m := &InformerManager{}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeExternalIP, Address: "192.168.1.10"}, {Type: corev1.NodeHostName, Address: "ofan-hostname"}, {Type: corev1.NodeInternalIP, Address: "192.168.20.69"}}}}
	slice := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "kubernetes", Namespace: "default", Labels: map[string]string{"kubernetes.io/service-name": "differentservice"}}, Endpoints: []discoveryv1.Endpoint{{Addresses: []string{"192.168.20.69"}}}}
	client := fake.NewSimpleClientset(node, slice)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := m.StartNodeAndEndpointInformer(client, ctx)
	require.NoError(t, err)

	require.True(t, cache.WaitForCacheSync(ctx.Done(), m.NodeInformer.HasSynced, m.EndpointInformer.HasSynced))
	m.nodeWatchError = errors.New("forbidden")

	health, err := m.NetworkHealth()
	require.NoError(t, err)

	assert.Equal(t, "drift", health.State)
	assert.Equal(t, health.NodeWatchError.Error(), "forbidden")
	assert.Equal(t, []string{"192.168.20.69"}, health.NodeIPs)
}
