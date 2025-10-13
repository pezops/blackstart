package kubernetes

import (
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

func init() {
	blackstart.RegisterModule("kubernetes_configmap", NewConfigMapModule)
}

var _ blackstart.Module = &configMapModule{}

func NewConfigMapModule() blackstart.Module {
	return &configMapModule{}
}

type configMap struct {
	cmi clientcorev1.ConfigMapInterface
	cm  *corev1.ConfigMap ``
}

func (c *configMap) Update(ctx blackstart.ModuleContext) error {
	var err error
	c.cm, err = c.cmi.Update(ctx, c.cm, metav1.UpdateOptions{})
	return err
}

func (c *configMap) Delete(ctx blackstart.ModuleContext) error {
	return c.cmi.Delete(ctx, c.cm.Name, metav1.DeleteOptions{})
}

type configMapModule struct{}

func (c *configMapModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "kubernetes_configmap",
		Name: "Kubernetes ConfigMap",
		Description: util.CleanString(
			`
Manages a Kubernetes ConfigMap resource, but not content.

**Notes**

- This module does not manage content of the ConfigMap. Use the '''kubernetes_configmap_value''' module 
  to manage key-value pairs in the ConfigMap.
- Once a ConfigMap is set to be immutable, values cannot be set or changed. Do not set a ConfigMap to be 
  immutable before setting the values. See [Immutable ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable) 
  for more information.
`,
		),
		Inputs: map[string]blackstart.InputValue{
			inputName: {
				Description: "Name of the ConfigMap",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputNamespace: {
				Description: "Namespace where the ConfigMap exists",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "default",
			},
			inputClient: {
				Description: "Kubernetes client interface to use for API calls",
				Type:        reflect.TypeFor[kubernetes.Interface](),
				Required:    true,
			},
			inputImmutable: {
				Description: "Make the ConfigMap immutable. Ignored if not set (default).",
				Type:        reflect.TypeFor[*bool](),
				Required:    false,
				Default:     nil,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConfigMap: {
				Description: "The ConfigMap resource",
				Type:        reflect.TypeFor[*configMap](),
			},
		},
		Examples: map[string]string{
			"Basic ConfigMap Usage": `id: create-configmap
module: kubernetes_configmap
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-config
  namespace: default`,
			"Configure ConfigMap to be Immutable": `operations:
  - id: k8s_client
    module: kubernetes_client
  - id: myapp_configmap
    module: kubernetes_configmap
    name: MyApp ConfigMap
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: myapp
      name: myapp-config
  - id: myapp_db_host
    module: kubernetes_configmap_value
    inputs:
      configmap:
        fromDependency:
          id: myapp_configmap
          output: configmap
      key: db_host
      value: db.myapp.svc.cluster.local
  - id: myapp_db_port
    module: kubernetes_configmap_value
    inputs:
      configmap:
        fromDependency:
          id: myapp_configmap
          output: configmap
      key: db_port
      value: "5432"
  - id: myapp_configmap_immutable
    module: kubernetes_configmap
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: myapp
      name: myapp-config
      immutable: true
    dependsOn:
      - myapp_db_host
      - myapp_db_port
`,
		},
	}
}

func (c *configMapModule) Validate(op blackstart.Operation) error {
	nameInput, ok := op.Inputs[inputName]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputName)
	}
	name := nameInput.String()
	if name == "" {
		return fmt.Errorf("input '%s' must be non-empty", inputName)
	}

	// Client is required
	_, ok = op.Inputs[inputClient]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputClient)
	}

	return nil
}

func (c *configMapModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	var cm *corev1.ConfigMap
	if ctx.Tainted() {
		return false, nil
	}

	clientInput, err := ctx.Input(inputClient)
	if err != nil {
		return false, fmt.Errorf("failed to get client input: %w", err)
	}

	cc, ok := clientInput.Any().(kubernetes.Interface)
	if !ok {
		return false, fmt.Errorf("client input is not a Kubernetes clientset")
	}

	nameInput, err := ctx.Input(inputName)
	if err != nil {
		return false, err
	}
	name := nameInput.String()

	namespaceInput, err := ctx.Input(inputNamespace)
	if err != nil {
		return false, err
	}
	namespace := namespaceInput.String()

	cmi := cc.CoreV1().ConfigMaps(namespace)

	cm, err = cmi.Get(ctx, name, metav1.GetOptions{})

	if ctx.DoesNotExist() {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	if err != nil {
		return false, err
	}

	// Get the ConfigMap
	if cm != nil {
		// Check if immutable field matches desired state (only if immutable input is provided)
		var immutableInput blackstart.Input
		immutableInput, err = ctx.Input(inputImmutable)
		if err != nil {
			return false, err
		}
		if immutableInput.Any() != nil {
			// immutableInput is provided and not nil
			desiredImmutable := immutableInput.Bool()
			if cm.Immutable == nil {
				return false, nil
			}
			currentImmutable := cm.Immutable

			if desiredImmutable != *currentImmutable {
				return false, nil
			}
		}

		err = ctx.Output("configmap", &configMap{cmi: cmi, cm: cm})
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (c *configMapModule) Set(ctx blackstart.ModuleContext) error {
	clientInput, err := ctx.Input(inputClient)
	if err != nil {
		return fmt.Errorf("failed to get client input: %w", err)
	}

	client, ok := clientInput.Any().(kubernetes.Interface)
	if !ok {
		return fmt.Errorf("client input is not a Kubernetes clientset")
	}

	nameInput, err := ctx.Input(inputName)
	if err != nil {
		return err
	}
	name := nameInput.String()

	namespaceInput, err := ctx.Input(inputNamespace)
	if err != nil {
		return err
	}
	namespace := namespaceInput.String()

	cmi := client.CoreV1().ConfigMaps(namespace)

	// If DoesNotExist is true, ensure the entire ConfigMap doesn't exist
	if ctx.DoesNotExist() {
		// Try to get the ConfigMap
		var cm *corev1.ConfigMap
		cm, err = cmi.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// ConfigMap doesn't exist
				return nil
			}
			return err
		}

		if cm != nil {
			// ConfigMap exists, delete it
			err = cmi.Delete(ctx, name, metav1.DeleteOptions{})
			return err
		}
		return fmt.Errorf("could not determine if ConfigMap '%s/%s' exists", namespace, name)
	}

	// Try to get the ConfigMap
	cm, err := cmi.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap doesn't exist
			newCm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string]string{},
			}

			// Only set immutable if the input is provided and not nil
			var immutableInput blackstart.Input
			immutableInput, err = ctx.Input(inputImmutable)
			if err != nil {
				return err
			}

			if immutableInput.Any() != nil {
				desiredImmutable := immutableInput.Bool()
				newCm.Immutable = &desiredImmutable
			}

			_, err = cmi.Create(ctx, newCm, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			return ctx.Output("configmap", &configMap{cmi: cmi, cm: newCm})
		}
		return err
	}

	// ConfigMap should exist
	if cm != nil {
		needsUpdate := false

		// Check if we need to update the immutable field (only if immutable input is provided)
		var immutableInput blackstart.Input
		immutableInput, err = ctx.Input(inputImmutable)
		if err != nil {
			return err
		}
		if immutableInput.Any() != nil {
			// immutableInput is provided and not nil
			desiredImmutable := immutableInput.Bool()
			currentImmutable := cm.Immutable

			if currentImmutable == nil || desiredImmutable != *currentImmutable {
				needsUpdate = true
				cm.Immutable = &desiredImmutable
			}
		}

		// Update the ConfigMap if needed
		if needsUpdate {
			cm, err = cmi.Update(ctx, cm, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}

		return ctx.Output("configmap", &configMap{cmi: cmi, cm: cm})
	}

	return fmt.Errorf("could not determine if ConfigMap '%s/%s' exists", namespace, name)
}
