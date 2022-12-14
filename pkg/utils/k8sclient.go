package utils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient is the wrapper of Kubernetes K8sClient that helps us easier to mock and test using Dependency Injection
type K8sClient struct {
	Clientset kubernetes.Interface
}

// IK8sClientHelper is the interface to get a Client
// Using this way, we can easily create a mock object that satisfies this interface
type IK8sClientHelper interface {
	GetClient(pathToCfg string) (*K8sClient, error)
	GetClientAndConfig(pathToCfg string) (*K8sClient, *rest.Config, error)
}

// ClienHelper is a helper class that implement IClientHelper, and returns the real Kubernetes Clientset
type K8sClientHelper struct {
}

// GetClientAndConfig returns a Kubernetes Clientset and the REST configuration which is built from a given config file path,
// for example `~/.kube/config`
// if the file path is empty, we will use the mode "InCluster"
// (with a token of serviec account stored in `/var/run/secrets/kubernetes.io/serviceaccount/token`)
// otherwise, use the server information and authentication information from the path
func (c K8sClientHelper) GetClientAndConfig(pathToCfg string) (*K8sClient, *rest.Config, error) {
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
		return nil, nil, err
	}

	if clientset, err := kubernetes.NewForConfig(config); err != nil {
		return nil, nil, err
	} else {
		return &K8sClient{Clientset: clientset}, config, nil
	}
}

// GetClient returns a Kubernetes Clientset which is built from a given config file path,
// for example `~/.kube/config`
// if the file path is empty, we will use the mode "InCluster"
// (with a token of serviec account stored in `/var/run/secrets/kubernetes.io/serviceaccount/token`)
// otherwise, use the server information and authentication information from the path
func (c K8sClientHelper) GetClient(pathToCfg string) (*K8sClient, error) {
	client, _, err := c.GetClientAndConfig(pathToCfg)
	return client, err
}
