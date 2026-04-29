package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart/api/v1alpha1"
	"gopkg.in/yaml.v3"
)

func TestDecodeOperationInputExtra_YAMLScalarString(t *testing.T) {
	v, err := decodeOperationInputExtra([]byte("bstest"))
	require.NoError(t, err)
	assert.Equal(t, "bstest", v)
}

func TestDecodeOperationInputExtra_Regression_PreviousJSONOnlyFailure(t *testing.T) {
	// This reproduces the previous failure path in loadOperations, which used
	// json.Unmarshal only for OperationInput.Extra.Raw values.
	var old any
	err := json.Unmarshal([]byte("bstest"), &old)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")

	// Current behavior: YAML scalar strings are accepted.
	v, err := decodeOperationInputExtra([]byte("bstest"))
	require.NoError(t, err)
	assert.Equal(t, "bstest", v)
}

func TestDecodeOperationInputExtra_JSONString(t *testing.T) {
	v, err := decodeOperationInputExtra([]byte(`"bstest"`))
	require.NoError(t, err)
	assert.Equal(t, "bstest", v)
}

func TestDecodeOperationInputExtra_PrimitivesAndMap(t *testing.T) {
	tests := []struct {
		name     string
		raw      []byte
		expected any
	}{
		{name: "bool", raw: []byte("true"), expected: true},
		{name: "number", raw: []byte("15"), expected: float64(15)},
		{name: "map", raw: []byte("foo: bar"), expected: map[string]any{"foo": "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := decodeOperationInputExtra(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, v)
		})
	}
}

func TestLoadOperations_YAMLScalarStringInput_Regression(t *testing.T) {
	// Reproduce real path:
	// YAML -> v1alpha1.OperationInput.Extra.Raw ("bstest") -> loadOperations.
	const wfYAML = `
name: test-cloudsql-mgmt
operations:
  - id: manage-instance
    module: google_cloudsql_managed_instance
    inputs:
      instance: bstest
`

	var cfg v1alpha1.WorkflowConfigFile
	err := yaml.Unmarshal([]byte(wfYAML), &cfg)
	require.NoError(t, err)
	require.Len(t, cfg.Operations, 1)

	// Confirm raw value shape that previously broke with json.Unmarshal-only logic.
	input := cfg.Operations[0].Inputs["instance"]
	require.NotNil(t, input)
	require.NotNil(t, input.Extra)
	assert.Equal(t, "bstest", string(input.Extra.Raw))

	ops, err := loadOperations(cfg.Operations)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	in := ops[0].Inputs["instance"]
	require.NotNil(t, in)
	v := in.Any()
	assert.Equal(t, "bstest", v)
}
