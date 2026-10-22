package k8s

import (
	"errors"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClientSet(kubeConfigPath string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err1 error
	var err2 error
	config, err1 = rest.InClusterConfig()
	if err1 != nil {
		kubeconfig := kubeConfigPath
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		}
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		config, err2 = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err2 != nil {
			configErrs := errors.Join(err1, err2)
			return nil, configErrs
		}
	}
	config.QPS = 50
	config.Burst = 100

	return kubernetes.NewForConfig(config)
}
