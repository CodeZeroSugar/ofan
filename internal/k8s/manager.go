package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var recreateDeadline = 150 * time.Second

type ServerManager struct {
	opts   ServerOpts
	client kubernetes.Interface
}

func NewServerManager(client kubernetes.Interface, opts ServerOpts) *ServerManager {
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

	_, err = m.client.CoreV1().Services(ns).Get(ctx, m.opts.Name+"-service", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	if err != nil {
		service := m.BuildService()
		_, err = m.client.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create service: %w", err)
		}
	}

	deployment, err := m.BuildDeployment()
	if err != nil {
		return fmt.Errorf("failed to build deployment for server '%s': %w", m.opts.Name, err)
	}
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

func (m *ServerManager) RecreateAll(ctx context.Context, sleep time.Duration) error {
	err := m.DeleteAll(ctx, false)
	if err != nil {
		return err
	}
	log.Printf("attempting to recreate '%s' for 150 seconds", m.opts.Name)
	ticker := time.NewTicker(recreateDeadline)
	defer ticker.Stop()

PollLoop:
	for {
		time.Sleep(sleep)
		select {
		case <-ticker.C:
			if _, err := m.client.AppsV1().Deployments(m.opts.Namespace).Get(ctx, m.opts.Name, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					break PollLoop
				}
			}
			return fmt.Errorf("deployment '%s' still terminating after wait deadline", m.opts.Name)
		default:
			if _, err := m.client.AppsV1().Deployments(m.opts.Namespace).Get(ctx, m.opts.Name, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					break PollLoop
				} else {
					return err
				}
			}
		}
	}
	return m.CreateAll(ctx)
}

func (m *ServerManager) ApplyConfig(ctx context.Context, hash string) error {
	cm := m.BuildConfigMap()
	s := m.BuildSecret()

	_, err := m.client.CoreV1().ConfigMaps(m.opts.Namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("could not find config map for server '%s' to update: %w", m.opts.Name, err)
		}
		return fmt.Errorf("failed to update config map for server '%s': %w", m.opts.Name, err)
	}

	_, err = m.client.CoreV1().Secrets(m.opts.Namespace).Update(ctx, s, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("could not find secret for server '%s' to update: %w", m.opts.Name, err)
		}
		return fmt.Errorf("failed to update secret for server '%s': %w", m.opts.Name, err)
	}

	dep, err := m.client.AppsV1().Deployments(m.opts.Namespace).Get(ctx, m.opts.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("could not find deployment for server '%s' to update: %w", m.opts.Name, err)
		}
		return fmt.Errorf("failed to update deployment for server '%s': %w", m.opts.Name, err)
	}

	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = make(map[string]string, 0)
	}
	dep.Spec.Template.Annotations[AnnotationConfigHash] = hash

	_, err = m.client.AppsV1().Deployments(m.opts.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("could not find deployment for server '%s' to update: %w", m.opts.Name, err)
		}
		return fmt.Errorf("failed to update deployment for server '%s': %w", m.opts.Name, err)
	}

	return nil
}
