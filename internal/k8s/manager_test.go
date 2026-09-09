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

func TestApplyConfig(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	mgr := NewServerManager(fakeClient, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-dev",
		Replicas:  1,
		Config:    DefaultValheimConfig("alpha", "secret123"),
	})
	ctx := context.Background()
	require.NoError(t, mgr.CreateAll(ctx))

	d, err := fakeClient.AppsV1().Deployments(mgr.opts.Namespace).Get(ctx, mgr.opts.Name, metav1.GetOptions{})
	require.NoError(t, err)
	d.Spec.Template.Annotations = nil
	_, err = fakeClient.AppsV1().Deployments(mgr.opts.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	require.NoError(t, err)

	cm, err := fakeClient.CoreV1().ConfigMaps(mgr.opts.Namespace).Get(ctx, mgr.opts.Name+"-configmap", metav1.GetOptions{})
	require.NoError(t, err)
	s, err := fakeClient.CoreV1().Secrets(mgr.opts.Namespace).Get(ctx, mgr.opts.Name+"-secret", metav1.GetOptions{})
	require.NoError(t, err)

	cmSrvPub1 := cm.Data["SERVER_PUBLIC"]
	s1 := s.StringData["SERVER_PASS"]

	mgr.opts.Config.CoreSettings.ServerPublic = true
	mgr.opts.Config.CoreSettings.ServerPass = "newsecretpass"
	h, err := hashCfg(mgr.opts.Config)
	require.NoError(t, err)

	require.NoError(t, mgr.ApplyConfig(ctx, h))

	cm, err = fakeClient.CoreV1().ConfigMaps(mgr.opts.Namespace).Get(ctx, mgr.opts.Name+"-configmap", metav1.GetOptions{})
	require.NoError(t, err)
	s, err = fakeClient.CoreV1().Secrets(mgr.opts.Namespace).Get(ctx, mgr.opts.Name+"-secret", metav1.GetOptions{})
	require.NoError(t, err)
	d, err = fakeClient.AppsV1().Deployments(mgr.opts.Namespace).Get(ctx, mgr.opts.Name, metav1.GetOptions{})
	require.NoError(t, err)

	require.NoError(t, err)

	assert.NotEqual(t, cmSrvPub1, cm.Data["SERVER_PUBLIC"])
	assert.NotEqual(t, s1, s.StringData["SERVER_PASS"])
	assert.Equal(t, "true", cm.Data["SERVER_PUBLIC"])
	assert.Equal(t, "newsecretpass", s.StringData["SERVER_PASS"])
	assert.Equal(t, h, d.Spec.Template.Annotations[AnnotationConfigHash])
}
