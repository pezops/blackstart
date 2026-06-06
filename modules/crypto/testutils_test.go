package crypto

import (
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// runModule runs an X.509 module and returns captured string outputs.
func runModule(
	t *testing.T, module blackstart.Module, moduleID string, inputs map[string]blackstart.Input,
) map[string]string {
	t.Helper()

	op := testOperation(moduleID, inputs)
	require.NoError(t, module.Validate(*op))
	ctx := &capturingModuleContext{ModuleContext: blackstart.OpContext(context.Background(), op)}
	ok, err := module.Check(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, module.Set(ctx))
	return stringOutputs(t, ctx.outputs)
}

// testOperation creates a module operation for tests.
func testOperation(moduleID string, inputs map[string]blackstart.Input) *blackstart.Operation {
	return &blackstart.Operation{Id: "test", Module: moduleID, Inputs: inputs}
}

// parseTestCSR parses a PEM-encoded CSR in tests.
func parseTestCSR(t *testing.T, value string) *stdx509.CertificateRequest {
	t.Helper()

	csr, err := parseCSRPEM(value)
	require.NoError(t, err)
	return csr
}

// parseTestCertificate parses a PEM-encoded certificate in tests.
func parseTestCertificate(t *testing.T, value string) *stdx509.Certificate {
	t.Helper()

	cert, err := parseCertificatePEM(value)
	require.NoError(t, err)
	return cert
}

// pemBlockCount counts PEM blocks of a given type.
func pemBlockCount(value, blockType string) int {
	remaining := []byte(value)
	count := 0
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			return count
		}
		if strings.EqualFold(block.Type, blockType) {
			count++
		}
		remaining = rest
	}
}
