package kubernetes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/pezops/blackstart"
)

func TestClientModule_Info(t *testing.T) {
	module := NewClientModule()
	info := module.Info()

	assert.Equal(t, "kubernetes_client", info.Id)

	// Check inputs
	_, exists := info.Inputs[inputContext]
	assert.True(t, exists)

	// Check outputs
	output, exists := info.Outputs[outputClient]
	assert.True(t, exists)
	assert.Equal(t, "kubernetes.Interface", output.Type.String())
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
				if test.expectError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			},
		)
	}
}

func TestClientModule_Check(t *testing.T) {
	module := NewClientModule()

	tests := []struct {
		name           string
		inputs         map[string]blackstart.Input
		expectedResult bool
	}{
		{
			name: "always returns false",
			inputs: map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue(""),
			},
			expectedResult: false,
		},
		{
			name: "with context returns false",
			inputs: map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue("test-context"),
			},
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(
			test.name, func(t *testing.T) {
				ctx := context.Background()
				moduleContext := blackstart.InputsToContext(ctx, test.inputs)

				result, err := module.Check(moduleContext)
				assert.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			},
		)
	}
}

func TestClientModule_Set(t *testing.T) {
	ctx := context.Background()

	cfg := setupEnvtest(t)

	clientset, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	_, err = clientset.Discovery().ServerVersion()
	require.NoError(t, err)

	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "kubeconfig")

	// Build a kubeconfig from the rest.Config
	kubeconfig := api.Config{
		Clusters: map[string]*api.Cluster{
			"envtest": {
				Server:                   cfg.Host,
				CertificateAuthorityData: cfg.CAData,
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"envtest": {
				ClientCertificateData: cfg.CertData,
				ClientKeyData:         cfg.KeyData,
			},
		},
		Contexts: map[string]*api.Context{
			"envtest": {
				Cluster:  "envtest",
				AuthInfo: "envtest",
			},
			"test-context": {
				Cluster:  "envtest",
				AuthInfo: "envtest",
			},
		},
		CurrentContext: "envtest",
	}

	// Write the kubeconfig to the temporary file
	err = clientcmd.WriteToFile(kubeconfig, kubeconfigPath)
	require.NoError(t, err)

	// Save the original KUBECONFIG env var to restore it later
	originalKubeconfig := os.Getenv("KUBECONFIG")
	t.Cleanup(
		func() {
			if originalKubeconfig != "" {
				_ = os.Setenv("KUBECONFIG", originalKubeconfig)
			} else {
				_ = os.Unsetenv("KUBECONFIG")
			}
		},
	)

	// Set KUBECONFIG to point to the temp file
	err = os.Setenv("KUBECONFIG", kubeconfigPath)
	require.NoError(t, err)

	module := NewClientModule()

	t.Run(
		"create client with default context", func(t *testing.T) {
			inputs := map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue(""),
			}

			moduleCtx := blackstart.InputsToContext(ctx, inputs)

			tErr := module.Set(moduleCtx)
			require.NoError(t, tErr)
		},
	)

	t.Run(
		"create client with specific context", func(t *testing.T) {
			inputs := map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue("test-context"),
			}

			moduleCtx := blackstart.InputsToContext(ctx, inputs)

			tErr := module.Set(moduleCtx)
			require.NoError(t, tErr)
		},
	)

	t.Run(
		"create client with nonexistent context", func(t *testing.T) {
			inputs := map[string]blackstart.Input{
				inputContext: blackstart.NewInputFromValue("nonexistent-context"),
			}

			moduleCtx := blackstart.InputsToContext(ctx, inputs)

			tErr := module.Set(moduleCtx)
			assert.Error(t, tErr)
		},
	)
}
