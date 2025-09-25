package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TestOperationFromYaml tests unmarshalling Operation resources from YAML.
func TestOperationFromYaml(t *testing.T) {

	tests := []struct {
		name string
		in   string
		out  Operation
	}{
		{
			name: "basic_0",
			in: `
module: "test_module"
id: test0
name: test0
description: test operation
`,
			out: Operation{
				Module:       "test_module",
				Id:           "test0",
				Name:         "test0",
				Description:  "test operation",
				DependsOn:    nil,
				Inputs:       nil,
				DoesNotExist: false,
				Tainted:      false,
			},
		},
		{
			name: "basic_1",
			in: `
module: "test_module"
id: test0
name: test0
does_not_exist: true
description: test operation
`,
			out: Operation{
				Module:       "test_module",
				Id:           "test0",
				Name:         "test0",
				Description:  "test operation",
				DependsOn:    nil,
				Inputs:       nil,
				DoesNotExist: true,
				Tainted:      false,
			},
		},
		{
			name: "with_inputs_0",
			in: `
module: "test_module"
id: test0
name: test0
description: test operation
inputs:
  foo: bar
`,
			out: Operation{
				Module:      "test_module",
				Id:          "test0",
				Name:        "test0",
				Description: "test operation",
				DependsOn:   nil,
				Inputs: map[string]*OperationInput{
					"foo": {Extra: &apiextensionsv1.JSON{Raw: []byte(`bar`)}},
				},
				DoesNotExist: false,
				Tainted:      false,
			},
		},
		{
			name: "with_inputs_1",
			in: `
module: "test_module"
id: test0
name: test0
description: test operation
inputs:
  foo: 
    from_dependency:
      id: asdf
      output: some_key
      
`,
			out: Operation{
				Module:      "test_module",
				Id:          "test0",
				Name:        "test0",
				Description: "test operation",
				DependsOn:   nil,
				Inputs: map[string]*OperationInput{
					"foo": {FromDependency: &FromDependency{Id: "asdf", Output: "some_key"}},
				},
				DoesNotExist: false,
				Tainted:      false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				var result Operation
				err := yaml.Unmarshal([]byte(tt.in), &result)
				assert.NoError(t, err)
				assert.Equal(t, tt.out, result)
			},
		)
	}
}

// TestWorkflowFromYaml tests unmarshalling Workflow resources from YAML.
func TestWorkflowFromYaml(t *testing.T) {
	{

		tests := []struct {
			name string
			in   string
			out  WorkflowConfigFile
		}{
			{
				name: "workflow_0",
				in: `
name: test Workflow
operations:
  - module: "test_module"
    id: test0
    name: test0
    description: test Operation
`,
				out: WorkflowConfigFile{
					Name: "test Workflow",
					WorkflowSpec: WorkflowSpec{
						Description: "",
						Operations: []Operation{
							{
								Module:       "test_module",
								Id:           "test0",
								Name:         "test0",
								Description:  "test Operation",
								DependsOn:    nil,
								Inputs:       nil,
								DoesNotExist: false,
								Tainted:      false,
							},
						},
					},
				},
			},
			{
				name: "workflow_1",
				in: `
name: test Workflow
operations:
- module: "test_module"
  id: test0
  name: test0
  description: test Operation
  inputs:
    foo: bar`,
				out: WorkflowConfigFile{
					Name: "test Workflow",
					WorkflowSpec: WorkflowSpec{
						Description: "",
						Operations: []Operation{
							{
								Module:      "test_module",
								Id:          "test0",
								Name:        "test0",
								Description: "test Operation",
								DependsOn:   nil,
								Inputs: map[string]*OperationInput{
									"foo": {Extra: &apiextensionsv1.JSON{Raw: []byte(`bar`)}},
								},
								DoesNotExist: false,
								Tainted:      false,
							},
						},
					},
				},
			},
			{
				name: "workflow_2",
				in: `
name: test Workflow
operations:
 - module: "test_module"
   id: test0
   name: test0
   description: test Operation
   inputs:
     foo:
       from_dependency:
         id: asdf
         output: some_key
 - module: "test_module"
   id: test1
   name: test1
   description: test Operation 1
   does_not_exist: true
   inputs:
     foo: bar
`,
				out: WorkflowConfigFile{
					Name: "test Workflow",
					WorkflowSpec: WorkflowSpec{
						Description: "",
						Operations: []Operation{
							{
								Module:      "test_module",
								Id:          "test0",
								Name:        "test0",
								Description: "test Operation",
								DependsOn:   nil,
								Inputs: map[string]*OperationInput{
									"foo": {FromDependency: &FromDependency{Id: "asdf", Output: "some_key"}},
								},
								DoesNotExist: false,
								Tainted:      false,
							},
							{
								Module:      "test_module",
								Id:          "test1",
								Name:        "test1",
								Description: "test Operation 1",
								DependsOn:   nil,
								Inputs: map[string]*OperationInput{
									"foo": {Extra: &apiextensionsv1.JSON{Raw: []byte(`bar`)}},
								},
								DoesNotExist: true,
								Tainted:      false,
							},
						},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(
				tt.name, func(t *testing.T) {
					var result WorkflowConfigFile
					var raw map[string]interface{}
					_ = yaml.Unmarshal([]byte(tt.in), &raw)
					err := yaml.Unmarshal([]byte(tt.in), &result)
					assert.NoError(t, err)
					assert.Equal(t, tt.out, result)
				},
			)
		}

	}
}

// TestInputFromYaml tests unmarshalling OperationInput resources from YAML.
func TestInputFromYaml(t *testing.T) {

	tests := []struct {
		name string
		in   string
		out  *OperationInput
	}{
		{
			name: "string_input",
			in:   "test",
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("test")}},
		},
		{
			name: "bool_input",
			in:   "true",
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("true")}},
		},
		{
			name: "int_input",
			in:   `15`,
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("15")}},
		},
		{
			name: "int64_input",
			in:   `9223372036854775807`,
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("9223372036854775807")}},
		},
		{
			name: "uint64_input",
			in:   `9223372036854775808`,
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("9223372036854775808")}},
		},
		{
			name: "long_input",
			in:   `18446744073709551616`,
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("1.8446744073709552e+19")}},
		},
		{
			name: "decimal_input",
			in:   `15.5`,
			out:  &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte("15.5")}},
		},
		{
			name: "quoted_string_input",
			in: `"
mapInput:
  foo: bar
"`,
			out: &OperationInput{Extra: &apiextensionsv1.JSON{Raw: []byte(`' mapInput: foo: bar '`)}},
		},
		{
			name: "from_dependency_input",
			in: `
from_dependency:
  id: foo
  output: bar
`,
			out: &OperationInput{FromDependency: &FromDependency{Id: "foo", Output: "bar"}},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				var result *OperationInput
				err := yaml.Unmarshal([]byte(tt.in), &result)
				assert.NoError(t, err)
				assert.Equal(t, tt.out, result)
			},
		)
	}
}
