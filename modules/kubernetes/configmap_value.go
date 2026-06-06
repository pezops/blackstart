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
		Description: "Manages key-value pairs in a Kubernetes ConfigMap resource.\n\n" + updatePolicyDocs,
		Requirements: []string{
			"The Kubernetes identity must be authorized to read and update ConfigMaps in the target namespace.",
			"Required ConfigMap verbs for this module: `get`, `update`.",
		},
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
				Description: "Value to set for the key. Required unless `update_policy` is `preserve_any`. Empty strings are allowed.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputUpdatePolicy: {
				Description: "Update policy for the key-value pair",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     updatePolicyPreserveAny,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputValue: {
				Description: "Current value stored for the key after reconciliation.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Read ConfigMap Value": `id: read-configmap-value
module: kubernetes_configmap_value
inputs:
  configmap:
    fromDependency:
      id: app-configmap
      output: configmap
  key: DATABASE_URL
  update_policy: preserve_any`,
			"Set ConfigMap Value": `id: set-configmap-example
module: kubernetes_configmap_value
inputs:
  configmap:
    fromDependency:
      id: app-configmap
      output: configmap
  key: DATABASE_URL
  value: postgres://user:password@localhost:5432/db
  update_policy: overwrite`,
		},
	}
}

func (c *configMapValueModule) Validate(op blackstart.Operation) error {
	// Key is required
	keyInput, ok := op.Inputs[inputKey]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputKey)
	}
	if keyInput.IsStatic() {
		key, err := blackstart.InputAs[string](keyInput, true)
		if err != nil {
			return fmt.Errorf("input '%s' is invalid: %w", inputKey, err)
		}
		if key == "" {
			return fmt.Errorf("input '%s' must be non-empty", inputKey)
		}
	}

	// ConfigMap is required
	_, ok = op.Inputs[inputConfigMap]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputConfigMap)
	}

	updatePolicy, policyKnown, err := operationUpdatePolicy(op)
	if err != nil {
		return err
	}
	if err = validateValueInput(op, updatePolicy, policyKnown); err != nil {
		return err
	}

	return nil
}

func (c *configMapValueModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	cmInput, err := ctx.Input(inputConfigMap)
	if err != nil {
		return false, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	cm, ok := cmInput.Any().(*configMap)
	if !ok {
		return false, fmt.Errorf("client input is not a ConfigMap")
	}

	key, err := blackstart.ContextInputAs[string](ctx, inputKey, true)
	if err != nil {
		return false, err
	}

	updatePolicy, err := contextUpdatePolicy(ctx)
	if err != nil {
		return false, err
	}

	desiredValue, hasValue, err := contextOptionalValue(ctx)
	if err != nil {
		return false, err
	}
	if err = requireValueInput(updatePolicy, hasValue); err != nil {
		return false, err
	}

	if ctx.Tainted() {
		return false, nil
	}

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

	switch updatePolicy {
	case updatePolicyOverwrite:
		if actualValue == desiredValue {
			return true, outputConfigMapValue(ctx, actualValue)
		}
		return false, nil
	case updatePolicyPreserve:
		if actualValue != "" {
			return true, outputConfigMapValue(ctx, actualValue)
		}
		return false, nil
	case updatePolicyPreserveAny:
		return true, outputConfigMapValue(ctx, actualValue)
	case updatePolicyFail:
		if actualValue != desiredValue {
			return false, fmt.Errorf(
				"key '%s' had a value changed, but updating the value is not allowed due to the update policy", key,
			)
		}
		return true, outputConfigMapValue(ctx, actualValue)
	}
	return false, fmt.Errorf("unhandled update policy: %s", updatePolicy)
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

	key, err := blackstart.ContextInputAs[string](ctx, inputKey, true)
	if err != nil {
		return err
	}

	desiredValue, hasValue, err := contextOptionalValue(ctx)
	if err != nil {
		return err
	}

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
		return nil
	}

	if err = requireSetValueInput(hasValue); err != nil {
		return err
	}

	// ConfigMap exists, update the value
	cm.cm.Data[key] = desiredValue

	if err = cm.Update(ctx); err != nil {
		return err
	}
	return outputConfigMapValue(ctx, desiredValue)
}

// outputConfigMapValue emits the value output for a ConfigMap key.
func outputConfigMapValue(ctx blackstart.ModuleContext, value string) error {
	return ctx.Output(outputValue, value)
}
