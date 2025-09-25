package blackstart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperationExecution(t *testing.T) {
	tests := []struct {
		name   string
		op     Operation
		result bool
	}{
		{
			name: "check_true_set_false",
			op: Operation{
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
			result: false,
		},
		{
			name: "check_true_set_true",
			op: Operation{
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
			result: true,
		},
		{
			name: "check_false_set_false",
			op: Operation{
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
			result: false,
		},
		{
			name: "check_false_set_true",
			op: Operation{
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
			result: true,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				mctx := newModuleContext(context.Background(), &tt.op)
				logger := NewLogger(nil)

				err := tt.op.setup()
				assert.NoError(t, err)
				err = tt.op.execute(mctx, logger.With("workflow", "test_workflow"))
				assert.NoError(t, err)
				assert.Equal(t, mctx.outputValues[testSetResult], tt.result)
			},
		)
	}
}
