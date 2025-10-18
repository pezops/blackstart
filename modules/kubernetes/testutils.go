package kubernetes

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func setupEnvtest(t *testing.T) *rest.Config {
	// Get envtest binaries path
	binPath, err := getEnvtestBinaries(t)
	if err != nil {
		t.Skipf("Skipping test: unable to get envtest binaries: %v", err)
	}

	// Start envtest with the binary path
	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: binPath,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = testEnv.Stop() })

	return cfg
}

// getEnvtestBinaries uses the setup-envtest CLI to download and return the path to envtest binaries
func getEnvtestBinaries(t *testing.T) (string, error) {
	cacheDir := getEnvtestCacheDir()

	// Ensure setup-envtest is installed
	setupEnvtestPath, err := exec.LookPath("setup-envtest")
	if err != nil {
		t.Logf("setup-envtest not found in PATH, attempting to install...")
		// Install setup-envtest
		installCmd := exec.Command("go", "install", "sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.22")
		output, installErr := installCmd.CombinedOutput()
		require.NoError(t, installErr, string(output))

		setupEnvtestPath, err = exec.LookPath("setup-envtest")
		require.NoError(t, err)
	}

	// Run setup-envtest use to get/download the latest binaries
	cmd := exec.Command(setupEnvtestPath, "use", "--bin-dir", cacheDir, "-p", "path")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	require.NoError(t, err)

	// The output is just the path, trimmed of whitespace
	binPath := strings.TrimSpace(stdout.String())
	require.NotEmpty(t, binPath)

	return binPath, nil
}

// getEnvtestCacheDir returns a consistent cache directory for envtest binaries
func getEnvtestCacheDir() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		// Fallback to temp dir
		return filepath.Join(os.TempDir(), "kubebuilder-envtest")
	}
	return filepath.Join(cacheDir, "kubebuilder-envtest")
}
