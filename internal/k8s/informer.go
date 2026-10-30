package k8s

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	discoveryv1 "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

type InformerManager struct {
	Registry           *ServerRegistry
	NodeLister         v1.NodeLister
	NodeInformer       cache.SharedIndexInformer
	nodeWatchError     error
	EndpointLister     discoveryv1.EndpointSliceLister
	EndpointInformer   cache.SharedIndexInformer
	endpointWatchError error
	mu                 sync.Mutex
}

type NetworkHealthResult struct {
	State              string
	NodeIPs            []string
	NodeWatchError     error
	EndpointIPs        []string
	EndpointWatchError error
	TimeChecked        time.Time
}

func StartInformerManager(clientset kubernetes.Interface, namespace string, ctx context.Context) (*InformerManager, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Minute, informers.WithNamespace(namespace))

	deployInformer := factory.Apps().V1().Deployments()
	serviceInformer := factory.Core().V1().Services()
	podInformer := factory.Core().V1().Pods()

	mgr := &InformerManager{
		Registry: NewServerRegistry(),
	}

	_, err := deployInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { mgr.upsertDeployment(obj.(*appsv1.Deployment)) },
		UpdateFunc: func(_, newObj interface{}) { mgr.upsertDeployment(newObj.(*appsv1.Deployment)) },
		DeleteFunc: mgr.deleteDeployment,
	})
	if err != nil {
		log.Printf("error occurred while registing deployment event handlers: %v", err)
	}
	_, err = serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { mgr.upsertService(obj.(*corev1.Service)) },
		UpdateFunc: func(_, newObj interface{}) { mgr.upsertService(newObj.(*corev1.Service)) },
		DeleteFunc: mgr.deleteService,
	})
	if err != nil {
		log.Printf("error occurred while registing service event handlers: %v", err)
	}
	_, err = podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { mgr.upsertPod(obj.(*corev1.Pod)) },
		UpdateFunc: func(_, newObj interface{}) { mgr.upsertPod(newObj.(*corev1.Pod)) },
		DeleteFunc: mgr.deletePod,
	})
	if err != nil {
		log.Printf("error occurred while registering pod event handlers: %v", err)
	}

	factory.Start(ctx.Done())

	synced := factory.WaitForCacheSync(ctx.Done())
	for informerType, ok := range synced {
		if !ok {
			return nil, fmt.Errorf("timed out waiting for %s cache sync", informerType)
		}
	}

	deps := deployInformer.Informer().GetStore().List()
	for _, d := range deps {
		obj, ok := d.(*appsv1.Deployment)
		if ok {
			mgr.upsertDeployment(obj)
		}
	}
	svcs := serviceInformer.Informer().GetStore().List()
	for _, s := range svcs {
		obj, ok := s.(*corev1.Service)
		if ok {
			mgr.upsertService(obj)
		}
	}
	pods := podInformer.Informer().GetStore().List()
	for _, p := range pods {
		obj, ok := p.(*corev1.Pod)
		if ok {
			mgr.upsertPod(obj)
		}
	}
	log.Println("informer synced, watching for events...")

	return mgr, nil
}

func (m *InformerManager) StartNodeAndEndpointInformer(clientset kubernetes.Interface, ctx context.Context) error {
	clusterFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	endpointFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Minute, informers.WithNamespace("default"))

	err := clusterFactory.Core().V1().Nodes().Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		m.mu.Lock()
		log.Printf("error occurred on %s: %v", r.TypeDescription(), err)
		m.nodeWatchError = err
		m.mu.Unlock()
	})
	if err != nil {
		return err
	}

	err = endpointFactory.Discovery().V1().EndpointSlices().Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		m.mu.Lock()
		log.Printf("error occurred on %s: %v", r.TypeDescription(), err)
		m.endpointWatchError = err
		m.mu.Unlock()
	})
	if err != nil {
		return err
	}

	m.NodeLister = clusterFactory.Core().V1().Nodes().Lister()
	m.NodeInformer = clusterFactory.Core().V1().Nodes().Informer()

	m.EndpointLister = endpointFactory.Discovery().V1().EndpointSlices().Lister()
	m.EndpointInformer = endpointFactory.Discovery().V1().EndpointSlices().Informer()

	clusterFactory.Start(ctx.Done())
	endpointFactory.Start(ctx.Done())

	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		syncRes := clusterFactory.WaitForCacheSyncWithContext(ctx)
		if syncRes.AsError() != nil {
			log.Printf("cluster factory failed to sync: %v", syncRes.AsError())
		} else {
			log.Println("cluster factory synced")
		}
	}()

	go func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		syncRes := endpointFactory.WaitForCacheSyncWithContext(ctx)
		if syncRes.AsError() != nil {
			log.Printf("endpoint factory failed to sync: %v", syncRes.AsError())
		} else {
			log.Println("endpoint factory synced")
		}
	}()

	return nil
}

func (m *InformerManager) getWatchErrors() map[string]error {
	m.mu.Lock()
	watchErrors := make(map[string]error)
	watchErrors["node"] = m.nodeWatchError
	watchErrors["endpoint"] = m.endpointWatchError
	m.mu.Unlock()
	return watchErrors
}

func (m *InformerManager) NetworkHealth() (*NetworkHealthResult, error) {
	var health NetworkHealthResult
	health.TimeChecked = time.Now()

	watchErrors := m.getWatchErrors()

	if m.NodeInformer == nil || m.NodeLister == nil || !m.NodeInformer.HasSynced() {
		health.State = "unknown"
		health.NodeWatchError = watchErrors["node"]
	}
	if m.EndpointInformer == nil || m.EndpointLister == nil || !m.EndpointInformer.HasSynced() {
		health.State = "unknown"
		health.EndpointWatchError = watchErrors["endpoint"]
	}

	if health.State == "unknown" {
		return &health, nil
	}

	nodes, err := m.NodeLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	addrs := make([]corev1.NodeAddress, 0)
	for _, node := range nodes {
		addrs = append(addrs, node.Status.Addresses...)
	}
	for _, addr := range addrs {
		if addr.Type == corev1.NodeInternalIP {
			health.NodeIPs = append(health.NodeIPs, addr.Address)
		}
	}
	if len(health.NodeIPs) == 0 {
		health.State = "unknown"
		return &health, nil
	}

	selector, err := labels.Parse("kubernetes.io/service-name=kubernetes")
	if err != nil {
		return nil, err
	}
	endpoints, err := m.EndpointLister.List(selector)
	if err != nil {
		return nil, err
	}
	for _, endpoint := range endpoints {
		for _, e := range endpoint.Endpoints {
			health.EndpointIPs = append(health.EndpointIPs, e.Addresses...)
		}
	}

	health.State = "drift"

	for _, ip := range health.NodeIPs {
		if slices.Contains(health.EndpointIPs, ip) {
			health.State = "ok"
			break
		}
	}

	health.NodeWatchError = watchErrors["node"]
	health.EndpointWatchError = watchErrors["endpoint"]

	return &health, nil
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

func (m *InformerManager) upsertPod(obj *corev1.Pod) {
	if obj.Labels[LabelManagedBy] != ManagedByOfan {
		return
	}
	name, ok := obj.Labels[LabelServerName]
	if !ok {
		return
	}
	m.Registry.UpdateIfExists(name, func(s *ServerState) {
		s.NodeIP = obj.Status.HostIP

		cs := obj.Status.ContainerStatuses
		if len(cs) > 0 {
			s.RestartCount = cs[0].RestartCount
			if cs[0].State.Running != nil {
				if cs[0].State.Running.StartedAt.Time.IsZero() {
					s.RunningSince = time.Time{}
				} else {
					s.RunningSince = cs[0].State.Running.StartedAt.Time
				}
				s.PodWaiting = ""
			} else if cs[0].State.Waiting != nil {
				s.RunningSince = time.Time{}
				s.PodWaiting = cs[0].State.Waiting.Reason
			} else {
				s.RunningSince = time.Time{}
				s.PodWaiting = ""
			}
		}
	})
}

func (m *InformerManager) deletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		tomb, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			return
		}
		pod, ok = tomb.Obj.(*corev1.Pod)
		if !ok {
			return
		}
	}
	if pod.Labels[LabelManagedBy] != ManagedByOfan {
		return
	}
	name, ok := pod.Labels[LabelServerName]
	if !ok {
		return
	}

	if ok = m.Registry.UpdateIfExists(name, func(s *ServerState) {
		s.NodeIP = ""
		s.PodWaiting = ""
		s.RestartCount = 0
		s.RunningSince = time.Time{}
	}); !ok {
		return
	}
	log.Printf("'%s' deleted, pod values zeroed in registry", pod.Name)
}
