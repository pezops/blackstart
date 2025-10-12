package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("Failed to start test environment: %v", err)
	}
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
		// Try to install setup-envtest
		installCmd := exec.Command("go", "install", "sigs.k8s.io/controller-runtime/tools/setup-envtest@latest")
		if output, err := installCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to install setup-envtest: %w\n%s", err, output)
		}

		// Try to find it again
		setupEnvtestPath, err = exec.LookPath("setup-envtest")
		if err != nil {
			return "", fmt.Errorf("setup-envtest not found even after installation: %w", err)
		}
	}

	// Run setup-envtest use to get/download the latest binaries
	cmd := exec.Command(setupEnvtestPath, "use", "--bin-dir", cacheDir, "-p", "path")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err = cmd.Run(); err != nil {
		return "", fmt.Errorf("setup-envtest failed: %w\nstderr: %s", err, stderr.String())
	}

	// The output is just the path, trimmed of whitespace
	binPath := strings.TrimSpace(stdout.String())
	if binPath == "" {
		return "", fmt.Errorf("failed to get binaries path from setup-envtest output: %s", stdout.String())
	}

	t.Logf("Using envtest binaries from: %s", binPath)
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
