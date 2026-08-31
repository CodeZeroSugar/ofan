package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const testNS = "ofan-test"

func newTestController(t *testing.T) (*db.Store, *fake.Clientset, *k8s.ServerRegistry, *Controller) {
	t.Helper()
	store, _ := db.NewStore(context.Background(), "file::memory:", "root", "testhash")
	t.Cleanup(func() { store.Close() })
	cs := fake.NewSimpleClientset()
	reg := k8s.NewServerRegistry()
	return store, cs, reg, NewController(store, cs, reg, testNS)
}

func configJSON(name string) string {
	b, err := json.Marshal(k8s.DefaultValheimConfig(name, "secret"))
	if err != nil {
		panic(err)
	}
	return string(b)
}

func seedRow(t *testing.T, store *db.Store, name, state string, purge bool) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateServer(ctx, name, "root", configJSON(name)); err != nil {
		t.Fatalf("failed to create row '%s': %v", name, err)
	}
	switch state {
	case "running":
	case "stopped":
		if err := store.UpdateState(ctx, name, "stopped"); err != nil {
			t.Fatalf("failed to set '%s' stopped: %v", name, err)
		}
	case "deleting":
		if err := store.MarkDeleting(ctx, name, purge); err != nil {
			t.Fatalf("failed to mark '%s' deleting: %v", name, err)
		}
	default:
		t.Fatalf("unknown desired state %q", state)
	}
}

func seedRowBadConfig(t *testing.T, store *db.Store, name, state string, purge bool) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateServer(ctx, name, "root", "where is json"); err != nil {
		t.Fatalf("failed to create row '%s': %v", name, err)
	}
	switch state {
	case "running":
	case "stopped":
		if err := store.UpdateState(ctx, name, "stopped"); err != nil {
			t.Fatalf("failed to set '%s' stopped: %v", name, err)
		}
	case "deleting":
		if err := store.MarkDeleting(ctx, name, purge); err != nil {
			t.Fatalf("failed to mark '%s' deleting: %v", name, err)
		}
	default:
		t.Fatalf("unknown desired state %q", state)
	}
}

func seedRegistry(t *testing.T, reg *k8s.ServerRegistry, name string, replicas int32) {
	t.Helper()
	reg.Upsert(name, func(s *k8s.ServerState) {
		s.Namespace = testNS
		s.Replicas = replicas
		s.Status = "running"
	})
}

func seedDeployment(t *testing.T, cs *fake.Clientset, name string, replicas int32) {
	t.Helper()
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
	}
	if _, err := cs.AppsV1().Deployments(testNS).Create(context.Background(), dep, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to see deployment '%s': %v", name, err)
	}
}

func seedFailures(t *testing.T, store *db.Store, name string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if err := store.IncrementFailure(ctx, name); err != nil {
			t.Fatalf("failed to increment failures for '%s': %v", name, err)
		}
	}
}

func getDeployment(t *testing.T, cs *fake.Clientset, name string) *appsv1.Deployment {
	t.Helper()
	d, err := cs.AppsV1().Deployments(testNS).Get(context.Background(),
		name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("deployment '%s' not found: %v", name, err)
	}
	return d
}

func deploymentExists(cs *fake.Clientset, name string) bool {
	_, err := cs.AppsV1().Deployments(testNS).Get(context.Background(), name, metav1.GetOptions{})
	return err == nil
}

func pvcExists(cs *fake.Clientset, name string) bool {
	_, err := cs.CoreV1().PersistentVolumeClaims(testNS).Get(context.Background(), name+"-pvc", metav1.GetOptions{})
	return err == nil
}

func serviceExists(cs *fake.Clientset, name string) bool {
	_, err := cs.CoreV1().Services(testNS).Get(context.Background(), name+"-service", metav1.GetOptions{})
	return err == nil
}

func secretExists(cs *fake.Clientset, name string) bool {
	_, err := cs.CoreV1().Secrets(testNS).Get(context.Background(), name+"-secret", metav1.GetOptions{})
	return err == nil
}

func configMapExists(cs *fake.Clientset, name string) bool {
	_, err := cs.CoreV1().ConfigMaps(testNS).Get(context.Background(), name+"-configmap", metav1.GetOptions{})
	return err == nil
}

func assertResourcesExist(t *testing.T, cs *fake.Clientset, name string) {
	t.Helper()
	assert.True(t, deploymentExists(cs, name))
	assert.True(t, pvcExists(cs, name))
	assert.True(t, serviceExists(cs, name))
	assert.True(t, secretExists(cs, name))
	assert.True(t, configMapExists(cs, name))
}

func assertResourcesNotExist(t *testing.T, cs *fake.Clientset, name string) {
	t.Helper()
	assert.False(t, deploymentExists(cs, name))
	assert.False(t, pvcExists(cs, name))
	assert.False(t, serviceExists(cs, name))
	assert.False(t, secretExists(cs, name))
	assert.False(t, configMapExists(cs, name))
}

func TestRunningRowConverges(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, 0, srv.ConsecutiveFailures)
}

func TestRunningRow_IdempotentSecondPass(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, 0, srv.ConsecutiveFailures)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	srv, err = s.GetServer(ctx, "alpha")
	require.NoError(t, err)

	assert.Equal(t, 0, srv.ConsecutiveFailures)
}

func TestRunningRow_ScalesReplicasToOpts(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedDeployment(t, client, "alpha", 0)
	seedRegistry(t, reg, "alpha", 0)

	assert.True(t, deploymentExists(client, "alpha"))
	dep, err := client.AppsV1().Deployments(testNS).Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	zeroPtr := int32(0)
	assert.Equal(t, &zeroPtr, dep.Spec.Replicas)

	st, exists := reg.Get("alpha")
	require.True(t, exists)
	require.NotNil(t, st)
	assert.Equal(t, int32(0), st.Replicas)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assert.True(t, deploymentExists(client, "alpha"))
	dep, err = client.AppsV1().Deployments(testNS).Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)

	onePtr := int32(1)
	assert.Equal(t, &onePtr, dep.Spec.Replicas)
}

func TestStoppedRow_ScalesToZero(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "stopped", false)
	seedDeployment(t, client, "alpha", 1)
	seedRegistry(t, reg, "alpha", 1)

	assert.True(t, deploymentExists(client, "alpha"))
	dep, err := client.AppsV1().Deployments(testNS).Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	onePtr := int32(1)
	assert.Equal(t, &onePtr, dep.Spec.Replicas)

	st, exists := reg.Get("alpha")
	require.True(t, exists)
	assert.Equal(t, int32(1), st.Replicas)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assert.True(t, deploymentExists(client, "alpha"))
	dep, err = client.AppsV1().Deployments(testNS).Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	zeroPtr := int32(0)
	assert.Equal(t, &zeroPtr, dep.Spec.Replicas)
}

func TestNoRegistryEntry_SkipsRow(t *testing.T) {
	ctx := context.Background()
	s, client, _, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")
}

func TestRunningRow_GateSkipsAt5(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	for i := 0; i < 5; i++ {
		err := s.IncrementFailure(ctx, "alpha")
		require.NoError(t, err)
	}
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 5, srv.ConsecutiveFailures)

	seedRegistry(t, reg, "alpha", 0)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")

	srv, err = s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 5, srv.ConsecutiveFailures)
}

func TestStoppedRow_GateSkipsAt5(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "stopped", false)
	for i := 0; i < 5; i++ {
		err := s.IncrementFailure(ctx, "alpha")
		require.NoError(t, err)
	}
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 5, srv.ConsecutiveFailures)

	seedRegistry(t, reg, "alpha", 0)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")

	srv, err = s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 5, srv.ConsecutiveFailures)
}

func TestCorruptConfig_IncrementsFailure(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRowBadConfig(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 0)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")

	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 1, srv.ConsecutiveFailures)
}

func TestSuccess_ResetsFailures(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	for i := 0; i < 3; i++ {
		err := s.IncrementFailure(ctx, "alpha")
		require.NoError(t, err)
	}
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 3, srv.ConsecutiveFailures)

	seedRegistry(t, reg, "alpha", 0)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	srv, err = s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 0, srv.ConsecutiveFailures)
}

func TestOneBadRow_DoesNotKillPass(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRowBadConfig(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)
	seedRow(t, s, "bravo", "running", false)
	seedRegistry(t, reg, "bravo", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	srvA, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	srvB, err := s.GetServer(ctx, "bravo")
	require.NoError(t, err)

	assert.Equal(t, 1, srvA.ConsecutiveFailures)
	assert.Equal(t, 0, srvB.ConsecutiveFailures)

	assertResourcesExist(t, client, "bravo")
}

func TestDeleting_TearsDownAndConsumes(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)
	seedDeployment(t, client, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	err = s.MarkDeleting(ctx, "alpha", false)
	require.NoError(t, err)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assert.False(t, deploymentExists(client, "alpha"))
	assert.False(t, serviceExists(client, "alpha"))
	assert.False(t, secretExists(client, "alpha"))
	assert.False(t, configMapExists(client, "alpha"))
	assert.True(t, pvcExists(client, "alpha"))

	_, err = s.GetServer(ctx, "alpha")
	assert.ErrorIs(t, err, db.ErrServerNotFound)
}

func TestDeleting_PurgesPVC(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", true)
	seedRegistry(t, reg, "alpha", 1)
	seedDeployment(t, client, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	err = s.MarkDeleting(ctx, "alpha", true)
	require.NoError(t, err)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")
	_, err = s.GetServer(ctx, "alpha")
	assert.ErrorIs(t, err, db.ErrServerNotFound)
}

func TestDeleting_PreservesPVC(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)
	seedDeployment(t, client, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesExist(t, client, "alpha")

	err = s.MarkDeleting(ctx, "alpha", false)
	require.NoError(t, err)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	assert.False(t, deploymentExists(client, "alpha"))
	assert.False(t, serviceExists(client, "alpha"))
	assert.False(t, secretExists(client, "alpha"))
	assert.False(t, configMapExists(client, "alpha"))
	assert.True(t, pvcExists(client, "alpha"))
}

func TestDeleting_BypassesFailureGate(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "deleting", true)
	seedFailures(t, s, "alpha", 5)
	seedRegistry(t, reg, "alpha", 1)
	seedDeployment(t, client, "alpha", 1)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	assertResourcesNotExist(t, client, "alpha")
	_, err = s.GetServer(ctx, "alpha")
	require.ErrorIs(t, err, db.ErrServerNotFound)
}

func TestOrphan_TornDownAfterThreePasses(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRegistry(t, reg, "alpha", 0)

	for i := 0; i < 2; i++ {
		err := c.Reconcile(ctx)
		require.NoError(t, err)
	}
	_, err := s.GetServer(ctx, "alpha")
	require.ErrorIs(t, err, db.ErrServerNotFound)

	require.NoError(t, c.Reconcile(ctx))
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, "deleting", srv.DesiredState)

	require.NoError(t, c.Reconcile(ctx))
	assert.False(t, deploymentExists(client, "alpha"))
	_, err = s.GetServer(ctx, "alpha")
	require.ErrorIs(t, err, db.ErrServerNotFound)
}

func TestRowPresence_ClearsDriftCount(t *testing.T) {
	ctx := context.Background()
	s, _, reg, c := newTestController(t)

	seedRegistry(t, reg, "alpha", 0)

	err := c.Reconcile(ctx)
	require.NoError(t, err)

	i, exists := c.driftCounts["alpha"]
	assert.True(t, exists)
	assert.True(t, i > 0)

	seedRow(t, s, "alpha", "running", false)

	err = c.Reconcile(ctx)
	require.NoError(t, err)

	_, exists = c.driftCounts["alpha"]
	assert.False(t, exists)
}

func TestPoke_NonBlocking(t *testing.T) {
	_, _, _, c := newTestController(t)

	c.Poke()
	done := make(chan struct{})

	go func() {
		c.Poke()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Poke blocked on a full trigger channel")
	}
}

func TestCreateAllError(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)
	client.PrependReactor("create", "deployments", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("faked api server error")
	})

	err := c.Reconcile(ctx)
	require.NoError(t, err)
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 1, srv.ConsecutiveFailures)
}

func TestEnsureReplicasError_IncrementsFailure(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 0)
	client.PrependReactor("get", "deployments", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("faked api server error")
	})

	require.NoError(t, c.Reconcile(ctx))
	srv, err := s.GetServer(ctx, "alpha")
	require.NoError(t, err)
	assert.Equal(t, 1, srv.ConsecutiveFailures)
}

func TestReconcilePass(t *testing.T) {
	ctx := context.Background()
	s, client, reg, c := newTestController(t)

	seedRow(t, s, "alpha", "running", false)
	seedRegistry(t, reg, "alpha", 1)

	c.reconcilePass(ctx)

	assertResourcesExist(t, client, "alpha")
}
