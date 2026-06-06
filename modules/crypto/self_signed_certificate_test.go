package crypto

import (
	"context"
	stdx509 "crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestSelfSignedCertificateModuleGeneratesServerCertificate verifies self-signed certificate output.
func TestSelfSignedCertificateModuleGeneratesServerCertificate(t *testing.T) {
	key := testED25519Key(t)
	outputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, key)),
			inputDNSNames:      blackstart.NewInputFromValue("app.example.com"),
		},
	)

	cert := parseTestCertificate(t, outputs[outputPEM])
	require.Equal(t, []string{"app.example.com"}, cert.DNSNames)
	require.Contains(t, cert.ExtKeyUsage, stdx509.ExtKeyUsageServerAuth)
	require.Equal(t, outputs[outputPEM], outputs[outputCombinedPEM])
	require.Regexp(t, `^[0-9a-f]{64}$`, outputs[outputSHA256])
	require.NotEmpty(t, outputs[outputSerialNumber])
	require.NotEmpty(t, outputs[outputNotBefore])
	require.NotEmpty(t, outputs[outputNotAfter])
}

// TestSelfSignedCertificateModuleGeneratesCACertificate verifies CA profile output.
func TestSelfSignedCertificateModuleGeneratesCACertificate(t *testing.T) {
	key := testRSAKey(t)
	outputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, key)),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
			inputCommonName:    blackstart.NewInputFromValue("Example CA"),
		},
	)

	cert := parseTestCertificate(t, outputs[outputPEM])
	require.True(t, cert.IsCA)
	require.NotZero(t, cert.KeyUsage&stdx509.KeyUsageCertSign)
}

// TestSelfSignedCertificateModuleValidateRejectsInvalidProfile verifies static profile validation.
func TestSelfSignedCertificateModuleValidateRejectsInvalidProfile(t *testing.T) {
	err := (&selfSignedCertificateModule{}).Validate(
		*testOperation(
			moduleIDSelfSignedCert,
			map[string]blackstart.Input{
				inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
				inputProfile:       blackstart.NewInputFromValue("bad"),
			},
		),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "allowed values")
}

// TestSelfSignedCertificateModuleRejectsDoesNotExist verifies deletion semantics are unsupported.
func TestSelfSignedCertificateModuleRejectsDoesNotExist(t *testing.T) {
	m := &selfSignedCertificateModule{}
	op := testOperation(
		moduleIDSelfSignedCert,
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
