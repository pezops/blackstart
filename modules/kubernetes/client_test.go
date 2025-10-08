package kubernetes

import (
	"testing"

	"github.com/pezops/blackstart"
)

func TestClientModule_Info(t *testing.T) {
	module := NewClientModule()
	info := module.Info()

	if info.Id != "kubernetes_client" {
		t.Errorf("Expected ID to be 'kubernetes_client', got '%s'", info.Id)
	}

	if _, exists := info.Inputs[inputContext]; !exists {
		t.Errorf("Expected input '%s' to exist", inputContext)
	}

	if output, exists := info.Outputs[outputClient]; !exists {
		t.Errorf("Expected output '%s' to exist", outputClient)
	} else if output.Type.String() != "kubernetes.Interface" {
		t.Errorf("Expected output type to be 'kubernetes.Interface', got '%s'", output.Type.String())
	}
}

func TestClientModule_Validate(t *testing.T) {
	module := NewClientModule()

	tests := []struct {
		name        string
		inputs      map[string]blackstart.Input
		expectError bool
	}{
		{
			name:        "empty inputs",
			inputs:      map[string]blackstart.Input{},
			expectError: false,
		},
		{
			name: "with context",
			inputs: map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue("test-context"),
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				operation := blackstart.Operation{
					Module: "kubernetes_client",
					Id:     "test",
					Inputs: test.inputs,
				}

				err := module.Validate(operation)
				if test.expectError && err == nil {
					t.Error("Expected error but got none")
				}
				if !test.expectError && err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			},
		)
	}
}
