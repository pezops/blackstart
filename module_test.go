package blackstart

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

const (
	testCheckResult = "check_result"
	testCheckError  = "check_error"
	testSetResult   = "set_result"
	testSetError    = "set_error"
)

var _ Module = testModule{}

func init() {
	RegisterModule("test_module", newTestModule)
}

type testModule struct {
}

func (t testModule) Info() ModuleInfo {
	return ModuleInfo{
		Id:          "test_module",
		Description: "A test module for blackstart.",
		Inputs: map[string]InputValue{
			testCheckResult: {
				Description: "Should the module check result",
				Type:        reflect.TypeOf(true),
				Required:    true,
			},
			testCheckError: {
				Description: "Should the module return an error on check",
				Type:        reflect.TypeOf(true),
				Required:    false,
				Default:     false,
			},
			testSetResult: {
				Description: "Should the module set result",
				Type:        reflect.TypeOf(true),
				Required:    true,
			},
			testSetError: {
				Description: "Should the module return an error on set",
				Type:        reflect.TypeOf(true),
				Required:    false,
				Default:     false,
			},
		},
		Outputs: map[string]OutputValue{
			"result": {
				Description: "The result of the module operation",
				Type:        reflect.TypeOf(""),
			},
		},
	}
}

func (t testModule) Validate(_ Operation) error {
	return nil
}

func (t testModule) Check(mctx ModuleContext) (bool, error) {
	cr, err := mctx.Input(testCheckResult)
	if err != nil {
		return false, err
	}

	ce, err := mctx.Input(testCheckError)
	if err != nil {
		return false, err
	}

	res := cr.Bool()
	if res == true {
		var sr Input
		var setRes bool
		sr, err = mctx.Input(testSetResult)
		if err != nil {
			// if the input doesn't exist, set the default result for Set() to true.
			setRes = true
		} else {
			setRes = sr.Bool()
		}

		err = mctx.Output(testSetResult, setRes)
		if err != nil {
			return false, err
		}
	}

	var input Input
	for i := 0; i < 10; i++ {
		input, err = mctx.Input(fmt.Sprintf("input_%d", i))
		// if the input doesn't exist, break the loop, but clear the error.
		if err != nil {
			err = nil
			break
		}
		var val any
		val, err = input.Auto()
		if err != nil {
			return false, err
		}
		if val == "foo" {
			err = mctx.Output(fmt.Sprintf("output_%d", i), "bar")
			if err != nil {
				return false, err
			}
			continue
		}
		err = mctx.Output(fmt.Sprintf("output_%d", i), val)
		if err != nil {
			return false, err
		}
	}

	if ce.Bool() {
		err = fmt.Errorf("test error on check")
	}

	return res, err
}

func (t testModule) Set(mctx ModuleContext) error {
	var res bool
	sr, err := mctx.Input(testSetResult)
	if err != nil {
		return err
	}

	se, err := mctx.Input(testSetError)
	if err != nil {
		return err
	}

	res = sr.Bool()
	err = mctx.Output(testSetResult, res)
	if err != nil {
		return err
	}

	var input Input
	for i := 0; i < 10; i++ {
		input, err = mctx.Input(fmt.Sprintf("input_%d", i))
		// if the input doesn't exist, break the loop, but clear the error.
		if err != nil {
			err = nil
			break
		}
		var val any
		val, err = input.Auto()
		if err != nil {
			return err
		}
		if val == "foo" {
			err = mctx.Output(fmt.Sprintf("output_%d", i), "bar")
			if err != nil {
				return err
			}
			continue
		}
		err = mctx.Output(fmt.Sprintf("output_%d", i), val)
		if err != nil {
			return err
		}
	}

	if se.Bool() {
		err = fmt.Errorf("test error on set")
	}

	return err
}

func newTestModule() Module {
	return &testModule{}
}

func TestDependencyOutputFromYaml(t *testing.T) {

	tests := []struct {
		name string
		in   string
		out  dependencyOutput
	}{
		{
			name: "output_config",
			in: `
id: foo
output: bar
`,
			out: dependencyOutput{
				OperationId: "foo",
				Output:      "bar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				var result dependencyOutput
				err := yaml.Unmarshal([]byte(tt.in), &result)
				assert.NoError(t, err)
				assert.Equal(t, tt.out, result)
			},
		)
	}
}
