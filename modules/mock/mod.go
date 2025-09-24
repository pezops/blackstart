package postgres

import (
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
)

const inputPass = "pass"

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
				Type:        reflect.TypeOf(true),
				Required:    false,
				Default:     true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Simple Mock": `id: mock-1
module: mock_module`,
		},
	}
}

func (g *moduleModule) Validate(_ blackstart.Operation) error {
	return nil
}

func (g *moduleModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	p, err := ctx.Input(inputPass)
	if err != nil {
		return false, err
	}
	return p.Bool(), nil
}

func (g *moduleModule) Set(ctx blackstart.ModuleContext) error {
	p, err := ctx.Input(inputPass)
	if err != nil {
		return err
	}
	if p.Bool() {
		return nil
	}

	return fmt.Errorf("mock module failed")
}
