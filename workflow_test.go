package blackstart

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testOpValues map[string]map[string]interface{}

func TestWorkflowExecution(t *testing.T) {
	tests := []struct {
		// name is the subtest name.
		name string

		// wf is the test Workflow to be executed. Currently, we only have the 'test_module' module
		// that is used in the test workflows.
		wf Workflow

		// checkOpInputs is a map of operation id to a map of input key to input value. This is used to
		// check the inputs to the specified operations in the test Workflow with expected values.
		checkOpInputs *testOpValues

		// checkOpOutputs is a map of operation id to a map of output key to output value. This is used to
		// check the outputs from the specified operations in the test Workflow with expected values.
		checkOpOutputs *testOpValues
	}{
		{
			name: "check_true_set_false",
			wf: Workflow{
				Name:        "",
				Description: "",
				Operations: []Operation{
					{
						Module:      "test_module",
						Id:          "test0",
						Name:        "test module",
						Description: "foo",
						DependsOn:   nil,
						Inputs: map[string]Input{
							testCheckResult: NewInputFromValue(true),
							testSetResult:   NewInputFromValue(false),
						},
						DoesNotExist: false,
						Tainted:      false,
					},
				},
			},
			checkOpInputs: &testOpValues{
				"test0": {
					testCheckResult: true,
				},
			},
			checkOpOutputs: &testOpValues{
				"test0": {
					testSetResult: false,
				},
			},
		},
		{
			name: "check_true_set_true",
			wf: Workflow{
				Name:        "",
				Description: "",
				Operations: []Operation{
					{
						Module:      "test_module",
						Id:          "test0",
						Name:        "test module",
						Description: "foo",
						DependsOn:   nil,
						Inputs: map[string]Input{
							testCheckResult: NewInputFromValue(true),
							testSetResult:   NewInputFromValue(true),
						},
						DoesNotExist: false,
						Tainted:      false,
					},
				},
			},
			checkOpInputs: &testOpValues{
				"test0": {
					testCheckResult: true,
				},
			},
			checkOpOutputs: &testOpValues{
				"test0": {
					testSetResult: true,
				},
			},
		},
		{
			name: "check_false_set_false",
			wf: Workflow{
				Name:        "",
				Description: "",
				Operations: []Operation{
					{
						Module:      "test_module",
						Id:          "test0",
						Name:        "test module",
						Description: "foo",
						DependsOn:   nil,
						Inputs: map[string]Input{
							testCheckResult: NewInputFromValue(false),
							testSetResult:   NewInputFromValue(false),
						},
						DoesNotExist: false,
						Tainted:      false,
					},
				},
			},
			checkOpInputs: &testOpValues{
				"test0": {
					testCheckResult: false,
				},
			},
			checkOpOutputs: &testOpValues{
				"test0": {
					testSetResult: false,
				},
			},
		},
		{
			name: "check_false_set_true",
			wf: Workflow{
				Name:        "",
				Description: "",
				Operations: []Operation{
					{
						Module:      "test_module",
						Id:          "test0",
						Name:        "test module",
						Description: "foo",
						DependsOn:   nil,
						Inputs: map[string]Input{
							testCheckResult: NewInputFromValue(false),
							testSetResult:   NewInputFromValue(true),
						},
						DoesNotExist: false,
						Tainted:      false,
					},
				},
			},
			checkOpInputs: &testOpValues{
				"test0": {
					testCheckResult: false,
				},
			},
			checkOpOutputs: &testOpValues{
				"test0": {
					testSetResult: true,
				},
			},
		},
		{
			name: "check_foo_set_bar",
			wf: Workflow{
				Name:        "",
				Description: "",
				Operations: []Operation{
					{
						Module:      "test_module",
						Id:          "test0",
						Name:        "test module",
						Description: "foo",
						DependsOn:   nil,
						Inputs: map[string]Input{
							"input_0":       NewInputFromValue("foo"),
							testCheckResult: NewInputFromValue(true),
							testSetResult:   NewInputFromValue(true),
						},
						DoesNotExist: false,
						Tainted:      false,
					},
				},
			},
			checkOpInputs: &testOpValues{
				"test0": {
					"input_0": "foo",
				},
			},
			checkOpOutputs: &testOpValues{
				"test0": {
					"output_0": "bar",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := context.Background()
				logger := NewTestingLogger()
				we := newWorkflowExecution(&tt.wf, logger)
				res := we.execute(ctx)
				assert.NoError(t, res.Err)

				// check inputs to specified operations in the test Workflow
				if tt.checkOpInputs != nil {
					for opId, iv := range *tt.checkOpInputs {
						for k, v := range iv {
							var input Input
							var err error
							input, err = we.opCtxs[opId].Input(k)
							assert.NoError(t, err)

							// normally, a module would need to implement the value conversion for inputs.
							switch v.(type) {
							case bool:
								convertedInput := input.Bool()
								assert.Equal(t, v, convertedInput)
							case string:
								convertedInput := input.String()
								assert.Equal(t, v, convertedInput)
							}
						}
					}
				}

				// check outputs from specified operations in the test Workflow
				if tt.checkOpOutputs != nil {
					for opId, iv := range *tt.checkOpOutputs {
						for k, v := range iv {
							var output interface{}
							output, _ = we.opCtxs[opId].getOutput(k)

							// normally, a module would need to implement the value conversion for inputs, here
							// we need to do it for the outputs just for test convenience.
							var ok bool
							switch v.(type) {
							case bool:
								var convertedOutput bool
								convertedOutput, ok = output.(bool)
								assert.True(t, ok)
								assert.Equal(t, v, convertedOutput)
							case string:
								var convertedOutput string
								convertedOutput, ok = output.(string)
								assert.True(t, ok)
								assert.Equal(t, v, convertedOutput)
							}

						}
					}
				}
			},
		)
	}
}

// TestOpoSort tests the topological sorting of operations into an expected order.
func TestOpoSort(t *testing.T) {
	tests := []struct {
		name   string
		in     []Operation
		out    []string
		outErr error
	}{
		{
			name: "sort_0",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test1"},
				},
			},
			out: []string{"test0", "test1", "test2"},
		},
		{
			name: "sort_1",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test2"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test0"},
				},
			},
			out: []string{"test0", "test2", "test1"},
		},
		// multi-level
		{
			name: "sort_2",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test3",
					DependsOn: []string{"test2"},
				},
			},
			out: []string{"test0", "test1", "test2", "test3"},
		},
		// Self-cycle
		{
			name: "sort_3",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test3",
					DependsOn: []string{"test3"},
				},
			},
			out:    nil,
			outErr: ErrOperationCycle,
		},
		// Circle
		{
			name: "sort_4",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: []string{"test3"},
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test1"},
				},
				{
					Id:        "test3",
					DependsOn: []string{"test2"},
				},
			},
			out:    nil,
			outErr: ErrOperationCycle,
		},
		// internal cycle
		{
			name: "sort_5",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0", "test3"},
				},
				{
					Id:        "test2",
					DependsOn: []string{"test1"},
				},
				{
					Id:        "test3",
					DependsOn: []string{"test2"},
				},
			},
			out:    nil,
			outErr: ErrOperationCycle,
		},
		// multiple start nodes
		{
			name: "sort_6",
			in: []Operation{
				{
					Id:        "test0",
					DependsOn: nil,
				},
				{
					Id:        "test1",
					DependsOn: []string{"test0", "test3"},
				},
				{
					Id:        "test2",
					DependsOn: nil,
				},
				{
					Id:        "test3",
					DependsOn: []string{"test2"},
				},
			},
			out:    []string{"test0", "test2", "test3", "test1"},
			outErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result, err := opoSort(tt.in)
				assert.Equal(t, tt.out, result)
				if tt.outErr != nil {
					assert.True(t, errors.Is(err, tt.outErr))
				}
			},
		)
	}
}
