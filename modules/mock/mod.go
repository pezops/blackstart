package mock

import (
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
)

const inputPass = "pass"
const inputTest = "test"

func init() {
	blackstart.RegisterModule("mock_module", NewModule)
}

var _ blackstart.Module = &moduleModule{}

func NewModule() blackstart.Module {
	return &moduleModule{}
}

type moduleModule struct{}

func (g *moduleModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "mock_module",
		Name:        "Mock Module",
		Description: "A mock module that does nothing. This module is used to mock operations and operation results for testing purposes.",
		Inputs: map[string]blackstart.InputValue{
			inputPass: {
				Description: "Determines if the operation should pass or fail.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     true,
			},
			inputTest: {
				Description: "Test-only input that accepts bool or string values.",
				Types:       []reflect.Type{reflect.TypeFor[bool](), reflect.TypeFor[string]()},
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Simple Mock": `id: mock-1
module: mock_module`,
		},
	}
}

func (g *moduleModule) Validate(op blackstart.Operation) error {
	passInput, ok := op.Inputs[inputPass]
	if !ok || !passInput.IsStatic() {
		return parseTestInputFromOperation(op)
	}
	_, err := parsePassInput(passInput)
	if err != nil {
		return err
	}
	return parseTestInputFromOperation(op)
}

func parsePassInput(input blackstart.Input) (bool, error) {
	if input == nil {
		return true, nil
	}

	if value, err := blackstart.InputAs[bool](input, false); err == nil {
		return value, nil
	}
	return false, fmt.Errorf("input %q must be bool", inputPass)
}

func parsePassFromContext(ctx blackstart.ModuleContext) (bool, error) {
	passInput, err := ctx.Input(inputPass)
	if err != nil {
		return true, nil
	}
	return parsePassInput(passInput)
}

func parseTestInput(input blackstart.Input) error {
	if input == nil {
		return nil
	}

	v := input.Any()
	switch v.(type) {
	case bool, string:
		return nil
	default:
		return fmt.Errorf("input %q must be bool or string", inputTest)
	}
}

func parseTestInputFromOperation(op blackstart.Operation) error {
	testInput, ok := op.Inputs[inputTest]
	if !ok || !testInput.IsStatic() {
		return nil
	}
	return parseTestInput(testInput)
}

func parseTestInputFromContext(ctx blackstart.ModuleContext) error {
	testInput, err := ctx.Input(inputTest)
	if err != nil {
		return nil
	}
	return parseTestInput(testInput)
}

func (g *moduleModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if err := parseTestInputFromContext(ctx); err != nil {
		return false, err
	}

	p, err := parsePassFromContext(ctx)
	if err != nil {
		return false, err
	}
	return p, nil
}

func (g *moduleModule) Set(ctx blackstart.ModuleContext) error {
	if err := parseTestInputFromContext(ctx); err != nil {
		return err
	}

	p, err := parsePassFromContext(ctx)
	if err != nil {
		return err
	}
	if p {
		return nil
	}

	return fmt.Errorf("mock module failed")
}
