package kubernetes

import (
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
)

func init() {
	blackstart.RegisterModule("kubernetes_configmap_value", NewConfigMapValueModule)
}

var _ blackstart.Module = &configMapValueModule{}

func NewConfigMapValueModule() blackstart.Module {
	return &configMapValueModule{}
}

type configMapValueModule struct{}

func (c *configMapValueModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "kubernetes_configmap_value",
		Name:        "Kubernetes ConfigMap Value",
		Description: "Manages key-value pairs in a Kubernetes ConfigMap resource.",
		Inputs: map[string]blackstart.InputValue{
			inputConfigMap: {
				Description: "ConfigMap resource",
				Type:        reflect.TypeFor[*configMap](),
				Required:    true,
			},
			inputKey: {
				Description: "Key in the ConfigMap to set",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputValue: {
				Description: "Value to set for the key",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Set ConfigMap Value": `id: set-configmap-example
module: kubernetes_configmap_value
inputs:
  configmap:
    fromDependency:
      id: app-configmap
      output: configmap
  key: DATABASE_URL
  value: postgres://user:password@localhost:5432/db`,
		},
	}
}

func (c *configMapValueModule) Validate(op blackstart.Operation) error {
	// Key is required
	keyInput, ok := op.Inputs[inputKey]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputKey)
	}
	key := keyInput.String()
	if key == "" {
		return fmt.Errorf("input '%s' must be non-empty", inputKey)
	}

	// Value is required (but can be an empty string)
	_, ok = op.Inputs[inputValue]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputValue)
	}

	// ConfigMap is required
	_, ok = op.Inputs[inputConfigMap]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputConfigMap)
	}

	return nil
}

func (c *configMapValueModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.Tainted() {
		return false, nil
	}

	cmInput, err := ctx.Input(inputConfigMap)
	if err != nil {
		return false, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	cm, ok := cmInput.Any().(*configMap)
	if !ok {
		return false, fmt.Errorf("client input is not a ConfigMap")
	}

	keyInput, err := ctx.Input(inputKey)
	if err != nil {
		return false, err
	}
	key := keyInput.String()

	desiredValueInput, err := ctx.Input(inputValue)
	if err != nil {
		return false, err
	}
	desiredValue := desiredValueInput.String()

	// If DoesNotExist is true, success is either the ConfigMap or key does not exist
	if ctx.DoesNotExist() {
		_, keyExists := cm.cm.Data[key]
		return !keyExists, nil
	}

	if cm.cm.Data == nil {
		return false, nil
	}

	actualValue, exists := cm.cm.Data[key]
	if !exists {
		return false, nil
	}

	return actualValue == desiredValue, nil
}

func (c *configMapValueModule) Set(ctx blackstart.ModuleContext) error {
	cmInput, err := ctx.Input(inputConfigMap)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	cm, ok := cmInput.Any().(*configMap)
	if !ok {
		return fmt.Errorf("client input is not a ConfigMap")
	}

	keyInput, err := ctx.Input(inputKey)
	if err != nil {
		return err
	}
	key := keyInput.String()

	desiredValueInput, err := ctx.Input(inputValue)
	if err != nil {
		return err
	}
	desiredValue := desiredValueInput.String()

	// ConfigMap exists, update the value
	if cm.cm.Data == nil {
		cm.cm.Data = make(map[string]string)
	}

	// If DoesNotExist is true, ensure the key doesn't exist
	if ctx.DoesNotExist() {
		if _, exists := cm.cm.Data[key]; exists {
			delete(cm.cm.Data, key)
			return cm.Update(ctx)
		}
	}

	// ConfigMap exists, update the value
	cm.cm.Data[key] = desiredValue

	return cm.Update(ctx)
}
