package main

import(
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strconv"
	"fmt"
)
type Client struct {
	clientset          *kubernetes.Clientset
	k8sVersion         float64
	namespace          string
	serviceAccountName string
}

func NewClient(config *rest.Config, ns, sa string) (*Client, error) {
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	version, err := cs.ServerVersion()
	if err != nil {
		return nil, err
	}

	vs, err := strconv.ParseFloat(fmt.Sprintf("%v.%v", version.Major, version.Minor), 64)
	if err != nil {
		return nil, err
	}

	return &Client{
		clientset:          cs,
		namespace:          ns,
		k8sVersion:         vs,
		serviceAccountName: sa,
	}, nil
}