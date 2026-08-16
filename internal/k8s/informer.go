package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type InformerManager struct {
	DeployLister  appslisters.DeploymentLister
	PodLister     corelisters.PodLister
	ServiceLister corelisters.ServiceLister
	PvcLister     corelisters.PersistentVolumeClaimLister
	db            *db.Store
}

func StartInformerManager(clientset kubernetes.Interface, namespace string, ctx context.Context, database *db.Store) (*InformerManager, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Minute, informers.WithNamespace(namespace))

	deployInformer := factory.Apps().V1().Deployments()
	podInformer := factory.Core().V1().Pods()
	serviceInformer := factory.Core().V1().Services()
	pvcInformer := factory.Core().V1().PersistentVolumeClaims()

	informers := map[string]cache.SharedIndexInformer{
		"deployment": deployInformer.Informer(),
		"pod":        podInformer.Informer(),
		"service":    serviceInformer.Informer(),
		"pvc":        pvcInformer.Informer(),
	}

	mgr := &InformerManager{
		DeployLister:  deployInformer.Lister(),
		PodLister:     podInformer.Lister(),
		ServiceLister: serviceInformer.Lister(),
		PvcLister:     pvcInformer.Lister(),
		db:            database,
	}

	for name, informer := range informers {
		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					log.Printf("%s ADDED: %s", name, key)
				}
			},
			UpdateFunc: func(oldObj interface{}, newObj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(newObj)
				if err == nil {
					log.Printf("%s UPDATED: %s", name, key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					log.Printf("%s DELETED: %s", name, key)
				}
			},
		})
		if err != nil {
			log.Printf("Error occurred adding event handlers for %s: %v", name, err)
			return nil, err
		}
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
