package kubernetes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pezops/blackstart"
)

func init() {
	blackstart.RegisterModule("kubernetes_secret_value", NewSecretValueModule)
}

var _ blackstart.Module = &secretValueModule{}

func NewSecretValueModule() blackstart.Module {
	return &secretValueModule{}
}

type secretValueModule struct{}

func (s *secretValueModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "kubernetes_secret_value",
		Name:        "Kubernetes Secret Value",
		Description: "Manages key-value pairs in a Kubernetes Secret resource.\n\n" + updatePolicyDocs,
		Requirements: []string{
			"The Kubernetes identity must be authorized to read and update Secrets in the target namespace.",
			"Required Secret verbs for this module: `get`, `update`.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputSecret: {
				Description: "Secret resource",
				Type:        reflect.TypeFor[*secret](),
				Required:    true,
			},
			inputKey: {
				Description: "Key in the Secret to set",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputValue: {
				Description: "Value to set for the key",
				Type:        reflect.TypeFor[string](),
				Required:    true,
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
			"Read Secret Value": `id: read-secret-value
module: kubernetes_secret_value
inputs:
  secret:
    fromDependency:
      id: app-secret
      output: secret
  key: DATABASE_PASSWORD
  value: ""
  update_policy: preserve_any`,
			"Set Secret Value": `id: set-secret-example
module: kubernetes_secret_value
inputs:
  secret:
    fromDependency:
      id: app-secret
      output: secret
  key: DATABASE_PASSWORD
  value: supersecretpassword
  update_policy: overwrite`,
		},
	}
}

func (s *secretValueModule) Validate(op blackstart.Operation) error {
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

	// Value is required (but can be an empty string)
	_, ok = op.Inputs[inputValue]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputValue)
	}

	// Secret is required
	_, ok = op.Inputs[inputSecret]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputSecret)
	}

	updatePolicy := updatePolicyPreserveAny
	if updatePolicyInput, exists := op.Inputs[inputUpdatePolicy]; exists {
		if !updatePolicyInput.IsStatic() {
			return nil
		}
		updatePolicyValue, err := blackstart.InputAs[string](updatePolicyInput, false)
		if err != nil {
			return fmt.Errorf("input '%s' is invalid: %w", inputUpdatePolicy, err)
		}
		updatePolicy = strings.TrimSpace(updatePolicyValue)
		if updatePolicy == "" {
			updatePolicy = updatePolicyPreserveAny
		}
	}

	_, ok = updatePolicies[updatePolicy]
	if !ok {
		return fmt.Errorf("input '%s' has invalid value '%s'", inputUpdatePolicy, updatePolicy)
	}

	return nil
}

func (s *secretValueModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	secInput, err := ctx.Input(inputSecret)
	if err != nil {
		return false, fmt.Errorf("failed to get Secret: %w", err)
	}

	sec, ok := secInput.Any().(*secret)
	if !ok {
		return false, fmt.Errorf("client input is not a Secret")
	}

	key, err := blackstart.ContextInputAs[string](ctx, inputKey, true)
	if err != nil {
		return false, err
	}

	desiredValue, err := blackstart.ContextInputAs[string](ctx, inputValue, true)
	if err != nil {
		return false, err
	}

	updatePolicy := updatePolicyPreserveAny
	updatePolicyInput, inputErr := blackstart.ContextInputAs[string](ctx, inputUpdatePolicy, false)
	if inputErr == nil {
		updatePolicy = strings.TrimSpace(updatePolicyInput)
	}
	if updatePolicy == "" {
		updatePolicy = updatePolicyPreserveAny
	}
	if _, ok := updatePolicies[updatePolicy]; !ok {
		return false, fmt.Errorf("input '%s' has invalid value '%s'", inputUpdatePolicy, updatePolicy)
	}

	if ctx.Tainted() {
		return false, nil
	}

	// If DoesNotExist is true, success is either the Secret or key does not exist
	if ctx.DoesNotExist() {
		_, keyExists := sec.s.Data[key]
		return !keyExists, nil
	}

	if sec.s.Data == nil {
		return false, nil
	}

	actualValueBytes, exists := sec.s.Data[key]
	if !exists {
		return false, nil
	}
	actualValue := string(actualValueBytes)

	switch updatePolicy {
	case updatePolicyOverwrite:
		if actualValue == desiredValue {
			return true, outputSecretValue(ctx, actualValue)
		}
		return false, nil
	case updatePolicyPreserve:
		if actualValue != "" {
			return true, outputSecretValue(ctx, actualValue)
		}
		return false, nil
	case updatePolicyPreserveAny:
		return true, outputSecretValue(ctx, actualValue)
	case updatePolicyFail:
		if actualValue != desiredValue {
			return false, fmt.Errorf(
				"key '%s' had a value changed, but updating the value is not allowed due to the update policy", key,
			)
		}
		return true, outputSecretValue(ctx, actualValue)
	}
	return false, fmt.Errorf("unhandled update policy: %s", updatePolicy)
}

func (s *secretValueModule) Set(ctx blackstart.ModuleContext) error {
	secInput, err := ctx.Input(inputSecret)
	if err != nil {
		return fmt.Errorf("failed to get Secret: %w", err)
	}

	sec, ok := secInput.Any().(*secret)
	if !ok {
		return fmt.Errorf("client input is not a Secret")
	}

	key, err := blackstart.ContextInputAs[string](ctx, inputKey, true)
	if err != nil {
		return err
	}

	desiredValue, err := blackstart.ContextInputAs[string](ctx, inputValue, true)
	if err != nil {
		return err
	}

	// Secret exists, update the value
	if sec.s.Data == nil {
		sec.s.Data = make(map[string][]byte)
	}

	// If DoesNotExist is true, ensure the key doesn't exist
	if ctx.DoesNotExist() {
		if _, exists := sec.s.Data[key]; exists {
			delete(sec.s.Data, key)
			return sec.Update(ctx)
		}
		return nil
	}

	// Secret exists, update the value
	sec.s.Data[key] = []byte(desiredValue)

	if err := sec.Update(ctx); err != nil {
		return err
	}
	return outputSecretValue(ctx, desiredValue)
}

// outputSecretValue emits the value output for a Secret key.
func outputSecretValue(ctx blackstart.ModuleContext, value string) error {
	return ctx.Output(outputValue, value)
}
