package kubernetes

import (
	"fmt"
	"reflect"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

type clusterConfigFunc func() (*rest.Config, error)

func init() {
	blackstart.RegisterModule("kubernetes_client", NewClientModule)
}

var _ blackstart.Module = &clientModule{}

func NewClientModule() blackstart.Module {
	return &clientModule{}
}

type clientModule struct{}

func (c *clientModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "kubernetes_client",
		Name:        "Kubernetes Client",
		Description: "Establishes a connection to a Kubernetes cluster and provides a client for other modules to use.",
		Inputs: map[string]blackstart.InputValue{
			inputContext: {
				Description: "The Kubernetes context to use. If not provided, uses the current-context from kubeconfig, or in-cluster config if running in a Kubernetes cluster.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputClient: {
				Description: "Kubernetes client that can be used by other modules.",
				Type:        reflect.TypeFor[kubernetes.Interface](),
			},
		},
		Examples: map[string]string{
			"Default Client": `id: default-k8s-client
module: kubernetes_client`,
			"Specific Context": `id: prod-k8s-client
module: kubernetes_client
inputs:
  context: prod-cluster`,
		},
	}
}

func (c *clientModule) Validate(_ blackstart.Operation) error {
	return nil
}

func (c *clientModule) Check(_ blackstart.ModuleContext) (bool, error) {
	return false, nil
}

func (c *clientModule) Set(ctx blackstart.ModuleContext) error {
	var kubeContext string
	var config *rest.Config
	var err error

	contextInput, err := ctx.Input(inputContext)
	if err != nil {
		return err
	}
	kubeContext = contextInput.String()

	// Attempt to do in-cluster configuration if no context is provided
	if kubeContext == "" {
		config, err = util.GetK8sClientConfig()
	} else {
		config, err = util.GetK8sClientConfigWithContext(kubeContext)
	}
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	// Make sure the connection is working
	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes cluster: %w", err)
	}

	// Set the client output
	err = ctx.Output(outputClient, clientsetAsInterface(clientset))
	if err != nil {
		return fmt.Errorf("failed to set client output: %w", err)
	}

	return nil
}

func clientsetAsInterface(clientset *kubernetes.Clientset) kubernetes.Interface {
	return clientset
}
