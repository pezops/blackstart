package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pezops/blackstart"
)

func init() {
	requirePrng()
}

// requirePrng ensures that the system has a secure PRNG available for the crypto/rand package.
func requirePrng() {
	if _, err := rand.Read(make([]byte, 1)); err != nil {
		panic("crypto/rand is unavailable")
	}
}

// RandomPassword generates a random password of the given length.
func RandomPassword(length int) string {
	const (
		lowerChars  = "abcdefghijklmnopqrstuvwxyz"
		upperChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		numberChars = "0123456789"
		symbolChars = "!@#$%^&*()-_=+[]{}<>?/|\\,.~;:"
	)
	allChars := lowerChars + upperChars + numberChars + symbolChars

	// Ensure the password contains at least one character from each required set
	password := make([]byte, length)
	password[0] = lowerChars[randInt(len(lowerChars))]
	password[1] = upperChars[randInt(len(upperChars))]
	password[2] = numberChars[randInt(len(numberChars))]
	password[3] = symbolChars[randInt(len(symbolChars))]

	for i := 4; i < length; i++ {
		idx := randInt(len(allChars))
		password[i] = allChars[idx]
	}

	return string(shuffle(password))
}

// shuffle randomly shuffles a byte slice.
func shuffle(b []byte) []byte {
	for i := range b {
		j := randInt(len(b))
		b[i], b[j] = b[j], b[i]
	}
	return b
}

// randInt generates a random integer in the range [0, max).
func randInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

// GetTestEnvRequiredVar retrieves a required environment variable for tests. If the variable is
// not set, the test is skipped.
func GetTestEnvRequiredVar(t testing.TB, modulePkg, key string) string {
	return getTestEnvVar(t, modulePkg, key, true)
}

// GetTestEnvOptionalVar retrieves an optional environment variable for tests.
func GetTestEnvOptionalVar(t testing.TB, modulePkg, key string) string {
	return getTestEnvVar(t, modulePkg, key, false)
}

// getTestEnvVar retrieves an environment variable for tests. If the variable is required and not set,
// the test is skipped.
func getTestEnvVar(t testing.TB, modulePkg, key string, required bool) string {
	modulePkgKey := strings.ReplaceAll(modulePkg, ".", "_")
	envKey := strings.ToUpper(strings.Join([]string{"BLACKSTART_TEST", modulePkgKey, key}, "_"))

	value := os.Getenv(envKey)
	if required && value == "" {
		t.Skip("skipping: test requires environment variable: " + envKey)
	}
	return value
}

// CleanString cleans up a string by removing leading and trailing newlines, and helps with using
// backticks in multi-line strings by replacing triple single quotes with backticks.
func CleanString(s string) string {
	s = strings.TrimPrefix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	s = strings.ReplaceAll(s, "'''", "`")
	return s
}

// GetK8sClientConfig creates a Kubernetes client config using the default loading rules.
func GetK8sClientConfig() (*rest.Config, error) {
	return GetK8sClientConfigWithContext("")
}

// GetK8sClientConfigWithContext creates a Kubernetes client config using the specified context.
func GetK8sClientConfigWithContext(kubeContext string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	var configOverrides *clientcmd.ConfigOverrides
	if kubeContext != "" {
		configOverrides = &clientcmd.ConfigOverrides{
			CurrentContext: kubeContext,
		}
	}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, configErr := kubeConfig.ClientConfig()
	if configErr != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client config: %w", configErr)
	}

	config = rest.AddUserAgent(config, blackstart.UserAgent)
	return config, nil
}
