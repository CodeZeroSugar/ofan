package k8s

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type InformerManager struct {
	Registry *ServerRegistry
}

func StartInformerManager(clientset kubernetes.Interface, namespace string, ctx context.Context) (*InformerManager, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Minute, informers.WithNamespace(namespace))

	deployInformer := factory.Apps().V1().Deployments()
	serviceInformer := factory.Core().V1().Services()

	mgr := &InformerManager{
		Registry: NewServerRegistry(),
	}

	_, err := deployInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { mgr.upsertDeployment(obj.(*appsv1.Deployment)) },
		UpdateFunc: func(_, newObj interface{}) { mgr.upsertDeployment(newObj.(*appsv1.Deployment)) },
		DeleteFunc: mgr.deleteDeployment,
	})
	if err != nil {
		log.Printf("error occurered while registing deployment event handlers: %v", err)
	}
	_, err = serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { mgr.upsertService(obj.(*corev1.Service)) },
		UpdateFunc: func(_, newObj interface{}) { mgr.upsertService(newObj.(*corev1.Service)) },
		DeleteFunc: mgr.deleteService,
	})
	if err != nil {
		log.Printf("error occurered while registing service event handlers: %v", err)
	}

	factory.Start(ctx.Done())

	synced := factory.WaitForCacheSync(ctx.Done())
	for informerType, ok := range synced {
		if !ok {
			return nil, fmt.Errorf("timed out waiting for %s cache sync", informerType)
		}
	}
	log.Println("informer synced, watching for events...")

	return mgr, nil
}

func (m *InformerManager) upsertDeployment(obj *appsv1.Deployment) {
	if obj.Labels[LabelManagedBy] != ManagedByOfan {
		return
	}
	m.Registry.Upsert(obj.Name, func(s *ServerState) {
		s.Status = deploymentStatus(obj)
		if obj.Spec.Replicas != nil {
			s.Replicas = *obj.Spec.Replicas
		}
		s.Ready = obj.Status.ReadyReplicas
		s.Namespace = obj.Namespace
	})
}

func (m *InformerManager) deleteDeployment(obj interface{}) {
	d, ok := obj.(*appsv1.Deployment)
	if !ok {
		tomb, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		d, ok = tomb.Obj.(*appsv1.Deployment)
		if !ok {
			return
		}
	}
	m.Registry.Delete(d.Name)
}

func (m *InformerManager) deleteService(obj interface{}) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		tomb, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		svc, ok = tomb.Obj.(*corev1.Service)
		if !ok {
			return
		}
	}
	if svc.Labels[LabelManagedBy] != ManagedByOfan {
		return
	}

	if ok = m.Registry.UpdateIfExists(strings.TrimSuffix(svc.Name, "-service"), func(s *ServerState) {
		s.NodePort = 0
		s.QueryPort = 0
	}); !ok {
		return
	}
	log.Printf("'%s' deleted, ports purged from registry entry", svc.Name)
}

func (m *InformerManager) upsertService(obj *corev1.Service) {
	if obj.Labels[LabelManagedBy] != ManagedByOfan {
		return
	}
	name := strings.TrimSuffix(obj.Name, "-service")
	var gp, qp int32
	for _, p := range obj.Spec.Ports {
		switch p.Name {
		case "valheim-udp":
			gp = p.NodePort
		case "valheim-query":
			qp = p.NodePort
		}
	}
	m.Registry.Upsert(name, func(s *ServerState) {
		s.NodePort = gp
		s.QueryPort = qp
	})
}
