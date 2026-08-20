package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateDeleteAllStoragePersist(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	mgr := NewServerManager(fakeClient, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-dev",
		Replicas:  1,
		Config:    DefaultValheimConfig("alpha", "secret123"),
	})
	ctx := context.Background()
	err := mgr.CreateAll(ctx)
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Namespaces().Get(ctx, "ofan-dev", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().PersistentVolumeClaims("ofan-dev").Get(ctx, "alpha-pvc", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Secrets("ofan-dev").Get(ctx, "alpha-secret", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().ConfigMaps("ofan-dev").Get(ctx, "alpha-configmap", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Services("ofan-dev").Get(ctx, "alpha-service", metav1.GetOptions{})
	assert.NoError(t, err)

	// Test: DeleteAll then confirm resources were cleaned up
	err = mgr.DeleteAll(ctx, false)
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Namespaces().Get(ctx, "ofan-dev", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().Secrets("ofan-dev").Get(ctx, "alpha-secret", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().ConfigMaps("ofan-dev").Get(ctx, "alpha-configmap", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().Services("ofan-dev").Get(ctx, "alpha-service", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))

	// PVC should still exist when deleteStorage=false
	_, err = fakeClient.CoreV1().PersistentVolumeClaims("ofan-dev").Get(ctx, "alpha-pvc", metav1.GetOptions{})
	assert.NoError(t, err)
}

func TestCreateDeleteAllStorageRemove(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	mgr := NewServerManager(fakeClient, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-dev",
		Replicas:  1,
		Config:    DefaultValheimConfig("alpha", "secret123"),
	})
	ctx := context.Background()
	err := mgr.CreateAll(ctx)
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Namespaces().Get(ctx, "ofan-dev", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().PersistentVolumeClaims("ofan-dev").Get(ctx, "alpha-pvc", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Secrets("ofan-dev").Get(ctx, "alpha-secret", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().ConfigMaps("ofan-dev").Get(ctx, "alpha-configmap", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Services("ofan-dev").Get(ctx, "alpha-service", metav1.GetOptions{})
	assert.NoError(t, err)

	// Test: DeleteAll then confirm resources were cleaned up
	err = mgr.DeleteAll(ctx, true)
	assert.NoError(t, err)
	_, err = fakeClient.CoreV1().Namespaces().Get(ctx, "ofan-dev", metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().Secrets("ofan-dev").Get(ctx, "alpha-secret", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().ConfigMaps("ofan-dev").Get(ctx, "alpha-configmap", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
	_, err = fakeClient.CoreV1().Services("ofan-dev").Get(ctx, "alpha-service", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))

	// PVC should NOT exist when deleteStorage=true
	_, err = fakeClient.CoreV1().PersistentVolumeClaims("ofan-dev").Get(ctx, "alpha-pvc", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestStopStart(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	mgr := NewServerManager(fakeClient, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-dev",
		Replicas:  1,
		Config:    DefaultValheimConfig("alpha", "secret123"),
	})
	ctx := context.Background()
	err := mgr.CreateAll(ctx)
	assert.NoError(t, err)

	// Test: Ensure stop sets replicas to 0
	zero := int32(0)
	err = mgr.Stop(ctx)
	require.NoError(t, err)
	dep, err := fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	assert.ObjectsAreEqual(&zero, dep.Spec.Replicas)

	// Test: Ensure start sets replicas to 1
	one := int32(1)
	err = mgr.Start(ctx)
	require.NoError(t, err)
	dep, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	assert.ObjectsAreEqual(&one, dep.Spec.Replicas)

	// Test: Ensure starting with a replica present is handled. replicas should not be greater than 1
	err = mgr.Start(ctx)
	assert.Error(t, err)
	dep, err = fakeClient.AppsV1().Deployments("ofan-dev").Get(ctx, "alpha", metav1.GetOptions{})
	require.NoError(t, err)
	assert.ObjectsAreEqual(&one, dep.Spec.Replicas)
}
