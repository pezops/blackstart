package kubernetes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pezops/blackstart"
)

// operationUpdatePolicy returns the static update policy for validation when it is known.
func operationUpdatePolicy(op blackstart.Operation) (string, bool, error) {
	input, exists := op.Inputs[inputUpdatePolicy]
	if !exists {
		return updatePolicyPreserveAny, true, nil
	}
	if !input.IsStatic() {
		return "", false, nil
	}
	value, err := blackstart.InputAs[string](input, false)
	if err != nil {
		return "", true, fmt.Errorf("input '%s' is invalid: %w", inputUpdatePolicy, err)
	}
	updatePolicy := strings.TrimSpace(value)
	if updatePolicy == "" {
		updatePolicy = updatePolicyPreserveAny
	}
	if _, ok := updatePolicies[updatePolicy]; !ok {
		return "", true, fmt.Errorf("input '%s' has invalid value '%s'", inputUpdatePolicy, updatePolicy)
	}
	return updatePolicy, true, nil
}

// validateValueInput validates a value input while allowing empty strings.
func validateValueInput(op blackstart.Operation, updatePolicy string, policyKnown bool) error {
	input, exists := op.Inputs[inputValue]
	if !exists {
		if policyKnown && updatePolicy != updatePolicyPreserveAny {
			return fmt.Errorf("input '%s' must be provided unless update_policy is '%s'", inputValue, updatePolicyPreserveAny)
		}
		return nil
	}
	if !input.IsStatic() {
		return nil
	}
	if input.Any() == nil {
		return fmt.Errorf("input '%s' must not be null", inputValue)
	}
	if _, err := blackstart.InputAs[string](input, false); err != nil {
		return fmt.Errorf("input '%s' is invalid: %w", inputValue, err)
	}
	return nil
}

// contextUpdatePolicy returns the runtime update policy from a module context.
func contextUpdatePolicy(ctx blackstart.ModuleContext) (string, error) {
	updatePolicy := updatePolicyPreserveAny
	updatePolicyInput, err := blackstart.ContextInputAs[string](ctx, inputUpdatePolicy, false)
	if err == nil {
		updatePolicy = strings.TrimSpace(updatePolicyInput)
	}
	if updatePolicy == "" {
		updatePolicy = updatePolicyPreserveAny
	}
	if _, ok := updatePolicies[updatePolicy]; !ok {
		return "", fmt.Errorf("input '%s' has invalid value '%s'", inputUpdatePolicy, updatePolicy)
	}
	return updatePolicy, nil
}

// contextOptionalValue returns a possibly omitted value input while allowing empty strings.
func contextOptionalValue(ctx blackstart.ModuleContext) (string, bool, error) {
	input, err := ctx.Input(inputValue)
	if err != nil {
		if errors.Is(err, blackstart.ErrInputDoesNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	if input.Any() == nil {
		return "", true, fmt.Errorf("input '%s' must not be null", inputValue)
	}
	value, err := blackstart.InputAs[string](input, false)
	if err != nil {
		return "", true, fmt.Errorf("input '%s' is invalid: %w", inputValue, err)
	}
	return value, true, nil
}

// requireValueInput enforces value presence for policies that need a desired value.
func requireValueInput(updatePolicy string, hasValue bool) error {
	if hasValue {
		return nil
	}
	if updatePolicy == updatePolicyPreserveAny {
		return nil
	}
	return fmt.Errorf("input '%s' must be provided unless update_policy is '%s'", inputValue, updatePolicyPreserveAny)
}

// requireSetValueInput enforces value presence when Set needs to write a key.
func requireSetValueInput(hasValue bool) error {
	if hasValue {
		return nil
	}
	return fmt.Errorf("input '%s' must be provided when setting a missing key", inputValue)
}
