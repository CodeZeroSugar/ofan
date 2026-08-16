package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ServerManager struct {
	opts   ServerOpts
	client *kubernetes.Clientset
}

func NewServerManager(client *kubernetes.Clientset, opts ServerOpts) *ServerManager {
	return &ServerManager{
		opts:   opts,
		client: client,
	}
}

func (m *ServerManager) CreateAll(ctx context.Context) error {
	ns := m.opts.Namespace

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}

	_, err := m.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	persistentVolumeClaim := m.BuildPersistentVolumeClaim()
	_, err = m.client.CoreV1().PersistentVolumeClaims(ns).Create(ctx, persistentVolumeClaim, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create persistent volume claim: %w", err)
	}

	secret := m.BuildSecret()
	_, err = m.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	configMap := m.BuildConfigMap()
	_, err = m.client.CoreV1().ConfigMaps(ns).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create config map: %w", err)
	}

	service := m.BuildService()
	_, err = m.client.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create service: %w", err)
	}

	deployment := m.BuildDeployment()
	_, err = m.client.AppsV1().Deployments(ns).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}

func (m *ServerManager) DeleteAll(ctx context.Context, deleteStorage bool) error {
	ns := m.opts.Namespace

	err := m.client.AppsV1().Deployments(ns).Delete(ctx, m.opts.Name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	err = m.client.CoreV1().Services(ns).Delete(ctx, m.opts.Name+"-service", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	err = m.client.CoreV1().ConfigMaps(ns).Delete(ctx, m.opts.Name+"-configmap", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete config map: %w", err)
	}

	err = m.client.CoreV1().Secrets(ns).Delete(ctx, m.opts.Name+"-secret", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	if deleteStorage {
		err := m.client.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, m.opts.Name+"-pvc", metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete persistent volume claim: %w", err)
		}
	}

	return nil
}

func (m *ServerManager) Stop(ctx context.Context) error {
	ns := m.opts.Namespace

	deployment, err := m.client.AppsV1().Deployments(ns).Get(ctx, m.opts.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to scale deployments to 0, could not get deployments: %w", err)
	}

	var zero int32 = 0
	deployment.Spec.Replicas = &zero

	_, err = m.client.AppsV1().Deployments(ns).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment replicas: %w", err)
	}

	return nil
}

func (m *ServerManager) Start(ctx context.Context) error {
	ns := m.opts.Namespace

	deployment, err := m.client.AppsV1().Deployments(ns).Get(ctx, m.opts.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to scale deployments to 1, could not get deployments: %w", err)
	}
	if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas > 0 {
		return fmt.Errorf("deployment already running with %d replicas", deployment.Status.Replicas)
	}

	deployment.Spec.Replicas = &maxReplicas

	_, err = m.client.AppsV1().Deployments(ns).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment replicas: %w", err)
	}

	return nil
}
