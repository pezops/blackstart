package kubernetes

import (
	"fmt"
	"reflect"

	"github.com/pezops/blackstart/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/pezops/blackstart"
)

func init() {
	blackstart.RegisterModule("kubernetes_secret", NewSecretModule)
}

var _ blackstart.Module = &secretModule{}

func NewSecretModule() blackstart.Module {
	return &secretModule{}
}

// secret is a helper struct that holds a functional Kubernetes Secret interface and Secret
// resource. This is passed to other modules so that dependent modules do not need both a client
// and a Secret resource to perform operations on a Secret.
type secret struct {
	// si is the Kubernetes Secret interface for API calls
	si clientcorev1.SecretInterface

	// s is the underlying Kubernetes Secret resource
	s *corev1.Secret
}

// Update updates the Secret resource in Kubernetes to match the current state of the Secret resource.
func (s *secret) Update(ctx blackstart.ModuleContext) error {
	var err error
	s.s, err = s.si.Update(ctx, s.s, metav1.UpdateOptions{})
	return err
}

// Delete deletes the Secret resource from Kubernetes.
func (s *secret) Delete(ctx blackstart.ModuleContext) error {
	return s.si.Delete(ctx, s.s.Name, metav1.DeleteOptions{})
}

// secretModule is a Blackstart module that manages a Kubernetes Secret resource.
type secretModule struct{}

func (s *secretModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "kubernetes_secret",
		Name: "Kubernetes Secret",
		Description: util.CleanString(
			`
Manages a Kubernetes Secret resource, but not content.

**Notes**

- This module does not manage content of the Secret. Use the '''kubernetes_secret_value''' module 
  to manage key-value pairs in the Secret.
- Once a secret is set to be immutable, values cannot be set or changed. Do not set a Secret to be 
  immutable before setting the values. See [Immutable Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#secret-immutable) 
  for more information.
`,
		),
		Inputs: map[string]blackstart.InputValue{
			inputName: {
				Description: "Name of the Secret",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputNamespace: {
				Description: "Namespace where the Secret exists",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "default",
			},
			inputClient: {
				Description: "Kubernetes client interface to use for API calls",
				Type:        reflect.TypeFor[kubernetes.Interface](),
				Required:    true,
			},
			inputType: {
				Description: "Type of the Secret (e.g., Opaque, kubernetes.io/tls, kubernetes.io/dockerconfigjson)",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "Opaque",
			},
			inputImmutable: {
				Description: "Whether the Secret should be immutable. When missing (default), immutability is not managed.",
				Type:        reflect.TypeFor[*bool](),
				Required:    false,
				Default:     nil,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputSecret: {
				Description: "The Secret resource",
				Type:        reflect.TypeFor[*secret](),
			},
		},
		Examples: map[string]string{
			"Create Secret": `id: create-secret
module: kubernetes_secret
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-secret
  namespace: default`,
			"Configure Secret to be Immutable": `id: immutable-secret
module: kubernetes_secret
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-immutable-secret
  namespace: default
  immutable: true`,
		},
	}
}

func (s *secretModule) Validate(op blackstart.Operation) error {
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

func (s *secretModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	var sec *corev1.Secret
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

	si := cc.CoreV1().Secrets(namespace)

	sec, err = si.Get(ctx, name, metav1.GetOptions{})

	if ctx.DoesNotExist() {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	if err != nil {
		return false, err
	}

	// Get the Secret
	if sec != nil {
		// Check if type matches desired state
		var typeInput blackstart.Input
		typeInput, err = ctx.Input(inputType)
		if err != nil {
			return false, err
		}

		desiredType := typeInput.String()
		if desiredType != "" {
			currentType := string(sec.Type)
			if currentType == "" {
				currentType = string(corev1.SecretTypeOpaque)
			}
			if currentType != desiredType {
				return false, nil
			}
		}

		// Check if immutable field matches desired state (only if immutable input is provided)
		var immutableInput blackstart.Input
		immutableInput, err = ctx.Input(inputImmutable)
		if err != nil {
			return false, err
		}
		if immutableInput.Any() != nil {
			// immutableInput is provided and not nil
			var desiredImmutablePtr *bool
			desiredImmutablePtr, ok = immutableInput.Any().(*bool)
			if ok && desiredImmutablePtr != nil {
				desiredImmutable := *desiredImmutablePtr
				currentImmutable := sec.Immutable != nil && *sec.Immutable

				if desiredImmutable != currentImmutable {
					return false, nil
				}
			}
		}

		err = ctx.Output("secret", &secret{si: si, s: sec})
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (s *secretModule) Set(ctx blackstart.ModuleContext) error {
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

	si := client.CoreV1().Secrets(namespace)

	// If DoesNotExist is true, ensure the entire Secret doesn't exist
	if ctx.DoesNotExist() {
		// Try to get the Secret
		var sec *corev1.Secret
		sec, err = si.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Secret doesn't exist
				return nil
			}
			return err
		}

		if sec != nil {
			// Secret exists, delete it
			return si.Delete(ctx, name, metav1.DeleteOptions{})
		}
		return fmt.Errorf("could not determine if Secret '%s/%s' exists", namespace, name)
	}

	// Try to get the Secret
	sec, err := si.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Secret doesn't exist
			newSec := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}

			// Set the secret type
			var typeInput blackstart.Input
			typeInput, err = ctx.Input(inputType)
			if err != nil {
				return err
			}
			newSec.Type = corev1.SecretType(typeInput.String())

			// Only set immutable if the input is provided and not nil
			var immutableInput blackstart.Input
			immutableInput, err = ctx.Input(inputImmutable)
			if err != nil {
				return err
			}

			if immutableInput.Any() != nil {
				var desiredImmutablePtr *bool
				if desiredImmutablePtr, ok = immutableInput.Any().(*bool); ok && desiredImmutablePtr != nil {
					newSec.Immutable = desiredImmutablePtr
				}
			}

			_, err = si.Create(ctx, newSec, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// Secret should exist
	if sec != nil {
		needsUpdate := false

		// Check if type needs to be updated
		var typeInput blackstart.Input
		typeInput, err = ctx.Input(inputType)
		if err != nil {
			return err
		}
		desiredType := corev1.SecretType(typeInput.String())
		if sec.Type != desiredType {
			needsUpdate = true
			sec.Type = desiredType
		}

		// Check if we need to update the immutable field (only if immutable input is provided)
		var immutableInput blackstart.Input
		immutableInput, err = ctx.Input(inputImmutable)
		if err != nil {
			return err
		}
		if immutableInput.Any() != nil {
			// immutableInput is provided and not nil
			var desiredImmutablePtr *bool
			desiredImmutablePtr, ok = immutableInput.Any().(*bool)
			if ok && desiredImmutablePtr != nil {
				desiredImmutable := *desiredImmutablePtr
				currentImmutable := sec.Immutable != nil && *sec.Immutable

				if desiredImmutable != currentImmutable {
					needsUpdate = true
					sec.Immutable = desiredImmutablePtr
				}
			}
		}

		// Update the Secret if needed
		if needsUpdate {
			sec, err = si.Update(ctx, sec, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}

		err = ctx.Output("secret", &secret{si: si, s: sec})
		if err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("could not determine if Secret '%s/%s' exists", namespace, name)
}
