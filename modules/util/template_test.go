package util_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/pezops/blackstart"
	_ "github.com/pezops/blackstart/modules/util"
	"github.com/stretchr/testify/require"
)

const (
	testEmitterModuleID = "test_value_emitter_module"
	testAssertModuleID  = "test_string_assert_module"
)

func init() {
	blackstart.RegisterModule(testEmitterModuleID, func() blackstart.Module { return &testValueEmitterModule{} })
	blackstart.RegisterModule(testAssertModuleID, func() blackstart.Module { return &testStringAssertModule{} })
}

type testValueEmitterModule struct{}

func (m *testValueEmitterModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id: testEmitterModuleID,
		Inputs: map[string]blackstart.InputValue{
			"username": {
				Type:     reflect.TypeFor[string](),
				Required: true,
			},
			"project_id": {
				Type:     reflect.TypeFor[string](),
				Required: true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			"username": {
				Type: reflect.TypeFor[string](),
			},
			"project_id": {
				Type: reflect.TypeFor[string](),
			},
		},
	}
}

func (m *testValueEmitterModule) Validate(op blackstart.Operation) error { return nil }
func (m *testValueEmitterModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	return false, nil
}
func (m *testValueEmitterModule) Set(ctx blackstart.ModuleContext) error {
	username, err := blackstart.ContextInputAs[string](ctx, "username", true)
	if err != nil {
		return err
	}
	projectID, err := blackstart.ContextInputAs[string](ctx, "project_id", true)
	if err != nil {
		return err
	}
	if err := ctx.Output("username", username); err != nil {
		return err
	}
	return ctx.Output("project_id", projectID)
}

type testStringAssertModule struct{}

func (m *testStringAssertModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id: testAssertModuleID,
		Inputs: map[string]blackstart.InputValue{
			"value": {
				Type:     reflect.TypeFor[string](),
				Required: true,
			},
			"expected": {
				Type:     reflect.TypeFor[string](),
				Required: true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
	}
}

func (m *testStringAssertModule) Validate(op blackstart.Operation) error { return nil }
func (m *testStringAssertModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	value, err := blackstart.ContextInputAs[string](ctx, "value", true)
	if err != nil {
		return false, err
	}
	expected, err := blackstart.ContextInputAs[string](ctx, "expected", true)
	if err != nil {
		return false, err
	}
	if value != expected {
		return false, fmt.Errorf("expected %q but got %q", expected, value)
	}
	return true, nil
}
func (m *testStringAssertModule) Set(ctx blackstart.ModuleContext) error { return nil }

func TestTemplateModule_RendersWorkflowOutputs(t *testing.T) {
	wf := blackstart.Workflow{
		Name: "util-template-workflow-output",
		Operations: []blackstart.Operation{
			{
				Id:     "producer",
				Module: testEmitterModuleID,
				Inputs: map[string]blackstart.Input{
					"username":   blackstart.NewInputFromValue("blackstart-sa"),
					"project_id": blackstart.NewInputFromValue("test-cr-249905"),
				},
			},
			{
				Id:        "templater",
				Module:    "util_template",
				DependsOn: []string{"producer"},
				Inputs: map[string]blackstart.Input{
					"template": blackstart.NewInputFromValue(
						`{{ workflowOutput "producer" "username" }}@{{ workflowOutput "producer" "project_id" }}.iam`,
					),
				},
			},
			{
				Id:        "assert",
				Module:    testAssertModuleID,
				DependsOn: []string{"templater"},
				Inputs: map[string]blackstart.Input{
					"value": blackstart.NewInputFromDep("templater", "result"),
					"expected": blackstart.NewInputFromValue(
						"blackstart-sa@test-cr-249905.iam",
					),
				},
			},
		},
	}

	result := wf.Run(context.Background())
	require.NoError(t, result.Err)
}

func TestTemplateModule_MissingWorkflowOutputFails(t *testing.T) {
	wf := blackstart.Workflow{
		Name: "util-template-missing-output",
		Operations: []blackstart.Operation{
			{
				Id:     "producer",
				Module: testEmitterModuleID,
				Inputs: map[string]blackstart.Input{
					"username":   blackstart.NewInputFromValue("blackstart-sa"),
					"project_id": blackstart.NewInputFromValue("test-cr-249905"),
				},
			},
			{
				Id:        "templater",
				Module:    "util_template",
				DependsOn: []string{"producer"},
				Inputs: map[string]blackstart.Input{
					"template": blackstart.NewInputFromValue(
						`{{ workflowOutput "producer" "missing_output" }}@example.iam`,
					),
				},
			},
		},
	}

	result := wf.Run(context.Background())
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "failed rendering template")
	require.Contains(t, result.Err.Error(), "missing_output")
}

func TestTemplateModule_WorkflowOutputRequiresDependsOn(t *testing.T) {
	wf := blackstart.Workflow{
		Name: "util-template-requires-depends-on",
		Operations: []blackstart.Operation{
			{
				Id:     "producer",
				Module: testEmitterModuleID,
				Inputs: map[string]blackstart.Input{
					"username":   blackstart.NewInputFromValue("blackstart-sa"),
					"project_id": blackstart.NewInputFromValue("test-cr-249905"),
				},
			},
			{
				Id:     "templater",
				Module: "util_template",
				Inputs: map[string]blackstart.Input{
					"template": blackstart.NewInputFromValue(
						`{{ workflowOutput "producer" "username" }}@example.iam`,
					),
				},
			},
		},
	}

	result := wf.Run(context.Background())
	require.Error(t, result.Err)
	require.Contains(t, result.Err.Error(), "not a declared dependency")
}
