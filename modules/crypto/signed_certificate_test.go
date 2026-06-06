package crypto

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestSignedCertificateModuleSignsCSR verifies local CA signing and chain outputs.
func TestSignedCertificateModuleSignsCSR(t *testing.T) {
	caKey := testED25519Key(t)
	caOutputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, caKey)),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
			inputCommonName:    blackstart.NewInputFromValue("Example CA"),
		},
	)
	leafKey := testECDSAKey(t)
	csrOutputs := runModule(
		t,
		&certificateRequestModule{},
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, leafKey)),
			inputDNSNames:      blackstart.NewInputFromValue([]string{"app.example.com"}),
		},
	)

	outputs := runModule(
		t,
		&signedCertificateModule{},
		moduleIDSignedCert,
		map[string]blackstart.Input{
			inputCSRPEM:           blackstart.NewInputFromValue(csrOutputs[outputPEM]),
			inputCACertificatePEM: blackstart.NewInputFromValue(caOutputs[outputPEM]),
			inputCAPrivateKeyPEM:  blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, caKey)),
			inputProfile:          blackstart.NewInputFromValue(profileServer),
		},
	)

	cert := parseTestCertificate(t, outputs[outputPEM])
	caCert := parseTestCertificate(t, caOutputs[outputPEM])
	require.NoError(t, cert.CheckSignatureFrom(caCert))
	require.Equal(t, caOutputs[outputPEM], outputs[outputChainPEM])
	require.Equal(t, outputs[outputPEM]+outputs[outputChainPEM], outputs[outputCombinedPEM])
	require.Equal(t, 2, pemBlockCount(outputs[outputCombinedPEM], pemTypeCertificate))
}

// TestSignedCertificateModuleAppendsCAChain verifies chain_pem and combined_pem ordering.
func TestSignedCertificateModuleAppendsCAChain(t *testing.T) {
	caKey := testED25519Key(t)
	caOutputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, caKey)),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
			inputCommonName:    blackstart.NewInputFromValue("Signing CA"),
		},
	)
	extraCAOutputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testED25519Key(t))),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
			inputCommonName:    blackstart.NewInputFromValue("Root CA"),
		},
	)
	csrOutputs := runModule(
		t,
		&certificateRequestModule{},
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
			inputDNSNames:      blackstart.NewInputFromValue("app.example.com"),
		},
	)

	outputs := runModule(
		t,
		&signedCertificateModule{},
		moduleIDSignedCert,
		map[string]blackstart.Input{
			inputCSRPEM:           blackstart.NewInputFromValue(csrOutputs[outputPEM]),
			inputCACertificatePEM: blackstart.NewInputFromValue(caOutputs[outputPEM]),
			inputCAPrivateKeyPEM:  blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, caKey)),
			inputCAChainPEM:       blackstart.NewInputFromValue(extraCAOutputs[outputPEM]),
		},
	)

	require.Equal(t, caOutputs[outputPEM]+extraCAOutputs[outputPEM], outputs[outputChainPEM])
	require.Equal(t, outputs[outputPEM]+outputs[outputChainPEM], outputs[outputCombinedPEM])
	require.Equal(t, 3, pemBlockCount(outputs[outputCombinedPEM], pemTypeCertificate))
}

// TestSignedCertificateModuleRejectsMismatchedCAKey verifies CA key and certificate mismatch errors.
func TestSignedCertificateModuleRejectsMismatchedCAKey(t *testing.T) {
	caKey := testRSAKey(t)
	otherKey := testRSAKey(t)
	caOutputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, caKey)),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
		},
	)
	csrOutputs := runModule(
		t,
		&certificateRequestModule{},
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
			inputDNSNames:      blackstart.NewInputFromValue("app.example.com"),
		},
	)

	m := &signedCertificateModule{}
	op := testOperation(
		moduleIDSignedCert,
		map[string]blackstart.Input{
			inputCSRPEM:           blackstart.NewInputFromValue(csrOutputs[outputPEM]),
			inputCACertificatePEM: blackstart.NewInputFromValue(caOutputs[outputPEM]),
			inputCAPrivateKeyPEM:  blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, otherKey)),
		},
	)
	ctx := blackstart.OpContext(context.Background(), op)
	ok, err := m.Check(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	err = m.Set(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")
}

// TestSignedCertificateModuleRejectsNonCACertificate verifies issuer certificates must be CAs.
func TestSignedCertificateModuleRejectsNonCACertificate(t *testing.T) {
	issuerKey := testRSAKey(t)
	issuerOutputs := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, issuerKey)),
			inputProfile:       blackstart.NewInputFromValue(profileServer),
			inputDNSNames:      blackstart.NewInputFromValue("issuer.example.com"),
		},
	)
	csrOutputs := runModule(
		t,
		&certificateRequestModule{},
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
			inputDNSNames:      blackstart.NewInputFromValue("app.example.com"),
		},
	)

	m := &signedCertificateModule{}
	op := testOperation(
		moduleIDSignedCert,
		map[string]blackstart.Input{
			inputCSRPEM:           blackstart.NewInputFromValue(csrOutputs[outputPEM]),
			inputCACertificatePEM: blackstart.NewInputFromValue(issuerOutputs[outputPEM]),
			inputCAPrivateKeyPEM:  blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, issuerKey)),
		},
	)
	ctx := blackstart.OpContext(context.Background(), op)
	ok, err := m.Check(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	err = m.Set(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a CA certificate")
}

// TestSignedCertificateValidateRequiredInputs verifies signed certificate validation failures.
func TestSignedCertificateValidateRequiredInputs(t *testing.T) {
	m := &signedCertificateModule{}
	validCSR := runModule(
		t,
		&certificateRequestModule{},
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
			inputDNSNames:      blackstart.NewInputFromValue("app.example.com"),
		},
	)[outputPEM]
	validCA := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
			inputProfile:       blackstart.NewInputFromValue(profileCA),
		},
	)[outputPEM]

	tests := []struct {
		name    string
		inputs  map[string]blackstart.Input
		wantErr string
	}{
		{name: "missing_csr", inputs: map[string]blackstart.Input{}, wantErr: inputCSRPEM},
		{name: "invalid_csr", inputs: map[string]blackstart.Input{inputCSRPEM: blackstart.NewInputFromValue("bad")}, wantErr: inputCSRPEM},
		{name: "missing_ca_cert", inputs: map[string]blackstart.Input{inputCSRPEM: blackstart.NewInputFromValue(validCSR)}, wantErr: inputCACertificatePEM},
		{
			name: "invalid_ca_cert",
			inputs: map[string]blackstart.Input{
				inputCSRPEM:           blackstart.NewInputFromValue(validCSR),
				inputCACertificatePEM: blackstart.NewInputFromValue("bad"),
			},
			wantErr: inputCACertificatePEM,
		},
		{
			name: "missing_ca_key",
			inputs: map[string]blackstart.Input{
				inputCSRPEM:           blackstart.NewInputFromValue(validCSR),
				inputCACertificatePEM: blackstart.NewInputFromValue(validCA),
			},
			wantErr: inputCAPrivateKeyPEM,
		},
		{
			name: "invalid_ca_key",
			inputs: map[string]blackstart.Input{
				inputCSRPEM:           blackstart.NewInputFromValue(validCSR),
				inputCACertificatePEM: blackstart.NewInputFromValue(validCA),
				inputCAPrivateKeyPEM:  blackstart.NewInputFromValue("bad"),
			},
			wantErr: inputCAPrivateKeyPEM,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := m.Validate(*testOperation(moduleIDSignedCert, tt.inputs))
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			},
		)
	}
}

// TestSignedCertificateModuleRejectsDoesNotExist verifies deletion semantics are unsupported.
func TestSignedCertificateModuleRejectsDoesNotExist(t *testing.T) {
	m := &signedCertificateModule{}
	op := testOperation(
		moduleIDSignedCert,
		map[string]blackstart.Input{
			inputCSRPEM:           blackstart.NewInputFromValue("placeholder"),
			inputCACertificatePEM: blackstart.NewInputFromValue("placeholder"),
			inputCAPrivateKeyPEM:  blackstart.NewInputFromValue("placeholder"),
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
