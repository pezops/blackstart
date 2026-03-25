package mock

import (
	"context"
	"reflect"
	"testing"

	"github.com/pezops/blackstart"
	"github.com/stretchr/testify/require"
)

func TestMockModule_InfoSupportsMultipleInputTypes(t *testing.T) {
	m := NewModule()
	info := m.Info()

	pass, ok := info.Inputs[inputPass]
	require.True(t, ok)
	require.Equal(t, reflect.TypeFor[bool](), pass.Type)
	require.Nil(t, pass.Types)

	testInput, ok := info.Inputs[inputTest]
	require.True(t, ok)
	require.Nil(t, testInput.Type)
	require.Len(t, testInput.Types, 2)
	require.Contains(t, testInput.Types, reflect.TypeFor[bool]())
	require.Contains(t, testInput.Types, reflect.TypeFor[string]())
}

func TestMockModule_CheckSupportsTestInputBoolAndString(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		want    bool
		wantErr bool
	}{
		{name: "bool", input: true, want: true},
		{name: "string", input: "ok", want: true},
		{name: "invalid type", input: 123, wantErr: true},
	}

	m := NewModule()
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := blackstart.InputsToContext(
					context.Background(),
					map[string]blackstart.Input{
						inputPass: blackstart.NewInputFromValue(true),
						inputTest: blackstart.NewInputFromValue(tt.input),
					},
				)
				got, err := m.Check(ctx)
				if tt.wantErr {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			},
		)
	}
}

func TestMockModule_SetSupportsTestInputBoolAndString(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantError bool
	}{
		{name: "bool", input: true, wantError: false},
		{name: "string", input: "value", wantError: false},
		{name: "invalid type", input: 123, wantError: true},
	}

	m := NewModule()
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := blackstart.InputsToContext(
					context.Background(),
					map[string]blackstart.Input{
						inputPass: blackstart.NewInputFromValue(true),
						inputTest: blackstart.NewInputFromValue(tt.input),
					},
				)
				err := m.Set(ctx)
				if tt.wantError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			},
		)
	}
}

func TestMockModule_ValidateSupportsBoolAndString(t *testing.T) {
	tests := []struct {
		name      string
		input     any
		wantError bool
	}{
		{name: "bool", input: true, wantError: false},
		{name: "string", input: "pass", wantError: false},
		{name: "invalid type", input: 123, wantError: true},
	}

	m := NewModule()
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				op := blackstart.Operation{
					Inputs: map[string]blackstart.Input{
						inputPass: blackstart.NewInputFromValue(true),
						inputTest: blackstart.NewInputFromValue(tt.input),
					},
				}
				err := m.Validate(op)
				if tt.wantError {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
			},
		)
	}
}
