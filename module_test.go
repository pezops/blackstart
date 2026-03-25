package blackstart

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				Type:        reflect.TypeFor[bool](),
				Required:    true,
			},
			testCheckError: {
				Description: "Should the module return an error on check",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
			testSetResult: {
				Description: "Should the module set result",
				Type:        reflect.TypeFor[bool](),
				Required:    true,
			},
			testSetError: {
				Description: "Should the module return an error on set",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
		},
		Outputs: map[string]OutputValue{
			"result": {
				Description: "The result of the module operation",
				Type:        reflect.TypeFor[string](),
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

	res, err := InputAs[bool](cr, true)
	if err != nil {
		return false, err
	}
	if res == true {
		var sr Input
		var setRes bool
		sr, err = mctx.Input(testSetResult)
		if err != nil {
			// if the input doesn't exist, set the default result for Set() to true.
			setRes = true
		} else {
			setRes, err = InputAs[bool](sr, true)
			if err != nil {
				return false, err
			}
		}

		err = mctx.Output(testSetResult, setRes)
		if err != nil {
			return false, err
		}
	}

	var input Input
	for i := 0; i < 10; i++ {
		input, err = mctx.Input(fmt.Sprintf("input_%d", i))
		if err != nil {
			if errors.Is(err, ErrInputDoesNotExist) {
				break
			}
			return false, err
		}
		if input == nil {
			break
		}
		val := input.Any()
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

	checkErr, err := InputAs[bool](ce, true)
	if err != nil {
		return false, err
	}
	if checkErr {
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

	res, err = InputAs[bool](sr, true)
	if err != nil {
		return err
	}
	err = mctx.Output(testSetResult, res)
	if err != nil {
		return err
	}

	var input Input
	for i := 0; i < 10; i++ {
		input, err = mctx.Input(fmt.Sprintf("input_%d", i))
		if err != nil {
			if errors.Is(err, ErrInputDoesNotExist) {
				break
			}
			return err
		}
		if input == nil {
			break
		}
		val := input.Any()
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

	setErr, err := InputAs[bool](se, true)
	if err != nil {
		return err
	}
	if setErr {
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

func TestValidateModuleInfo_RejectsTypeAndTypesTogether(t *testing.T) {
	info := ModuleInfo{
		Inputs: map[string]InputValue{
			"value": {
				Type:  reflect.TypeFor[bool](),
				Types: []reflect.Type{reflect.TypeFor[bool]()},
			},
		},
	}

	err := validateModuleInfo(info)
	require.Error(t, err)
	require.ErrorContains(t, err, "defines both Type and Types")
}

func TestValidateModuleInfo_RejectsNilOnlyTypes(t *testing.T) {
	info := ModuleInfo{
		Inputs: map[string]InputValue{
			"value": {
				Types: []reflect.Type{nil, nil},
			},
		},
	}

	err := validateModuleInfo(info)
	require.Error(t, err)
	require.ErrorContains(t, err, "all entries are nil")
}

type invalidModuleForRegistration struct{}

func (invalidModuleForRegistration) Info() ModuleInfo {
	return ModuleInfo{
		Id: "invalid_module_for_registration",
		Inputs: map[string]InputValue{
			"value": {
				Type:  reflect.TypeFor[bool](),
				Types: []reflect.Type{reflect.TypeFor[string]()},
			},
		},
	}
}

func (invalidModuleForRegistration) Validate(Operation) error { return nil }
func (invalidModuleForRegistration) Check(ModuleContext) (bool, error) {
	return true, nil
}
func (invalidModuleForRegistration) Set(ModuleContext) error { return nil }

func TestRegisterModule_PanicsForInvalidInputSchema(t *testing.T) {
	require.PanicsWithError(
		t,
		`invalid module registration "invalid_module_registration_test": input "value" defines both Type and Types; set only one`,
		func() {
			RegisterModule(
				"invalid_module_registration_test",
				func() Module { return invalidModuleForRegistration{} },
			)
		},
	)
}
