package blackstart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInputAs_StringSliceFromAny(t *testing.T) {
	in := NewInputFromValue([]any{"SELECT", "UPDATE"})
	v, err := InputAs[[]string](in, true)
	require.NoError(t, err)
	require.Equal(t, []string{"SELECT", "UPDATE"}, v)
}

func TestInputAs_RequiredString(t *testing.T) {
	_, err := InputAs[string](NewInputFromValue(""), true)
	require.Error(t, err)
}

func TestContextInputAs(t *testing.T) {
	ctx := InputsToContext(
		context.Background(),
		map[string]Input{
			"name": NewInputFromValue("blackstart"),
		},
	)
	name, err := ContextInputAs[string](ctx, "name", true)
	require.NoError(t, err)
	require.Equal(t, "blackstart", name)
}

func TestInputAs_BoolToBoolPtr(t *testing.T) {
	in := NewInputFromValue(true)
	v, err := InputAs[*bool](in, true)
	require.NoError(t, err)
	require.NotNil(t, v)
	require.True(t, *v)
}

func TestInputAs_SliceCoercions_Primitives(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "AnySliceToIntSlice",
			run: func(t *testing.T) {
				in := NewInputFromValue([]any{1, 2, 3})
				v, err := InputAs[[]int](in, true)
				require.NoError(t, err)
				require.Equal(t, []int{1, 2, 3}, v)
			},
		},
		{
			name: "AnySliceToInt64Slice",
			run: func(t *testing.T) {
				in := NewInputFromValue([]any{1, 2, 3})
				v, err := InputAs[[]int64](in, true)
				require.NoError(t, err)
				require.Equal(t, []int64{1, 2, 3}, v)
			},
		},
		{
			name: "AnySliceToFloat64Slice",
			run: func(t *testing.T) {
				in := NewInputFromValue([]any{1.5, 2.25, 3.75})
				v, err := InputAs[[]float64](in, true)
				require.NoError(t, err)
				require.Equal(t, []float64{1.5, 2.25, 3.75}, v)
			},
		},
		{
			name: "AnyNumericSliceToFloat64Slice",
			run: func(t *testing.T) {
				in := NewInputFromValue([]any{1, 2, 3})
				v, err := InputAs[[]float64](in, true)
				require.NoError(t, err)
				require.Equal(t, []float64{1, 2, 3}, v)
			},
		},
		{
			name: "AnySliceToBoolSlice",
			run: func(t *testing.T) {
				in := NewInputFromValue([]any{true, false, true})
				v, err := InputAs[[]bool](in, true)
				require.NoError(t, err)
				require.Equal(t, []bool{true, false, true}, v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func TestInputAs_SliceCoercions_InvalidCases(t *testing.T) {
	tests := []struct {
		name  string
		input any
		run   func(t *testing.T, in Input)
	}{
		{
			name:  "AnySliceToIntSlice_WithString",
			input: []any{1, "two"},
			run: func(t *testing.T, in Input) {
				_, err := InputAs[[]int](in, true)
				require.Error(t, err)
			},
		},
		{
			name:  "AnySliceToBoolSlice_WithInt",
			input: []any{true, 1},
			run: func(t *testing.T, in Input) {
				_, err := InputAs[[]bool](in, true)
				require.Error(t, err)
			},
		},
		{
			name:  "AnySliceToStringSlice_WithInt",
			input: []any{"one", 2},
			run: func(t *testing.T, in Input) {
				_, err := InputAs[[]string](in, true)
				require.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := NewInputFromValue(tt.input)
			tt.run(t, in)
		})
	}
}
