package crypto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestCertificateRequestModuleGeneratesCSRs verifies CSR generation for supported key algorithms.
func TestCertificateRequestModuleGeneratesCSRs(t *testing.T) {
	tests := []struct {
		name string
		key  any
	}{
		{name: "rsa", key: testRSAKey(t)},
		{name: "ecdsa", key: testECDSAKey(t)},
		{name: "ed25519", key: testED25519Key(t)},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				outputs := runModule(
					t,
					&certificateRequestModule{},
					moduleIDCertificateRequest,
					map[string]blackstart.Input{
						inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, tt.key)),
						inputDNSNames:      blackstart.NewInputFromValue([]string{"app.example.com"}),
					},
				)

				csr := parseTestCSR(t, outputs[outputPEM])
				require.Equal(t, []string{"app.example.com"}, csr.DNSNames)
				require.NoError(t, csr.CheckSignature())
			},
		)
	}
}

// TestCertificateRequestModuleRejectsDoesNotExist verifies deletion semantics are unsupported.
func TestCertificateRequestModuleRejectsDoesNotExist(t *testing.T) {
	m := &certificateRequestModule{}
	op := testOperation(
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
		},
	)
	op.DoesNotExist = true
	ctx := blackstart.OpContext(context.Background(), op)

	ok, err := m.Check(ctx)
	require.False(t, ok)
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesNotExist is not supported")

	err = m.Set(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesNotExist is not supported")
}
