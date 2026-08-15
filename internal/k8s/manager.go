package k8s

import (
	"context"
	"fmt"

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

	secret := m.BuildSecret()
	_, err := m.client.CoreV1().Secrets(ns).Create(ctx, secret, metav1.CreateOptions{})
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
