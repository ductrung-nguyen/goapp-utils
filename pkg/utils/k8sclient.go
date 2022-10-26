package utils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8Client is the wrapper of Kubernetes K8Client that helps us easier to mock and test using Dependency Injection
type K8Client struct {
	Clientset kubernetes.Interface
}

// IK8sClientHelper is the interface to get a Client
// Using this way, we can easily create a mock object that satisfies this interface
type IK8sClientHelper interface {
	GetClient(pathToCfg string) (*K8Client, error)
}

// ClienHelper is a helper class that implement IClientHelper, and returns the real Kubernetes Clientset
type K8sClientHelper struct {
}

// GetClient returns a Kubernetes Clientset which is built from a given config file path,
// for example `~/.kube/config`
// if the file path is empty, we will use the mode "InCluster"
// (with a token of serviec account stored in `/var/run/secrets/kubernetes.io/serviceaccount/token`)
// otherwise, use the server information and authentication information from the path
func (c K8sClientHelper) GetClient(pathToCfg string) (*K8Client, error) {
	var config *rest.Config
	var err error
	if pathToCfg == "" {
		// in cluster access
		config, err = rest.InClusterConfig()
	} else {
		// out of cluster
		config, err = clientcmd.BuildConfigFromFlags("", pathToCfg)
	}
	if err != nil {
		return nil, err
	}

	if clientset, err := kubernetes.NewForConfig(config); err != nil {
		return nil, err
	} else {
		return &K8Client{Clientset: clientset}, nil
	}
}
