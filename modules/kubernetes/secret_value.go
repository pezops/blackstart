package kubernetes

import (
	"fmt"
	"reflect"

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
		Description: "Manages key-value pairs in a Kubernetes Secret resource.",
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
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Set Secret Value": `id: set-secret-example
module: kubernetes_secret_value
inputs:
  secret:
    fromDependency:
      id: app-secret
      output: secret
  key: DATABASE_PASSWORD
  value: supersecretpassword`,
		},
	}
}

func (s *secretValueModule) Validate(op blackstart.Operation) error {
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

	// Secret is required
	_, ok = op.Inputs[inputSecret]
	if !ok {
		return fmt.Errorf("input '%s' must be provided", inputSecret)
	}

	return nil
}

func (s *secretValueModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.Tainted() {
		return false, nil
	}

	secInput, err := ctx.Input(inputSecret)
	if err != nil {
		return false, fmt.Errorf("failed to get Secret: %w", err)
	}

	sec, ok := secInput.Any().(*secret)
	if !ok {
		return false, fmt.Errorf("client input is not a Secret")
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

	// If DoesNotExist is true, success is either the Secret or key does not exist
	if ctx.DoesNotExist() {
		_, keyExists := sec.s.Data[key]
		return !keyExists, nil
	}

	if sec.s.Data == nil {
		return false, nil
	}

	actualValue, exists := sec.s.Data[key]
	if !exists {
		return false, nil
	}

	return string(actualValue) == desiredValue, nil
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
	}

	// Secret exists, update the value
	sec.s.Data[key] = []byte(desiredValue)

	return sec.Update(ctx)
}
