package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestPublicKeyValidate verifies public key module input validation.
func TestPublicKeyValidate(t *testing.T) {
	rsaKey := testRSAKey(t)
	privateKeyPEM := encodeTestPrivateKeyPEM(t, rsaKey)
	m := publicKeyModule{}

	tests := []struct {
		name    string
		inputs  map[string]blackstart.Input
		wantErr string
	}{
		{
			name:    "missing_private_key",
			inputs:  map[string]blackstart.Input{},
			wantErr: "missing required parameter: private_key_pem",
		},
		{
			name: "malformed_private_key",
			inputs: map[string]blackstart.Input{
				inputPrivateKeyPEM: blackstart.NewInputFromValue("not pem"),
			},
			wantErr: "failed parsing private key PEM",
		},
		{
			name: "valid_private_key",
			inputs: map[string]blackstart.Input{
				inputPrivateKeyPEM: blackstart.NewInputFromValue(privateKeyPEM),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Validate(blackstart.Operation{Id: "public", Module: moduleIDPublicKey, Inputs: tt.inputs})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPublicKeyCheckRejectsDoesNotExist verifies doesNotExist is unsupported.
func TestPublicKeyCheckRejectsDoesNotExist(t *testing.T) {
	rsaKey := testRSAKey(t)
	op := publicKeyOperation(encodeTestPrivateKeyPEM(t, rsaKey))
	op.DoesNotExist = true

	m := publicKeyModule{}
	ok, err := m.Check(blackstart.OpContext(context.Background(), op))
	require.False(t, ok)
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesNotExist is not supported")
}

// TestPublicKeySetDerivesPKCS8RSA verifies public key derivation from PKCS#8 RSA.
func TestPublicKeySetDerivesPKCS8RSA(t *testing.T) {
	key := testRSAKey(t)
	privateKeyPEM := encodeTestPrivateKeyPEM(t, key)

	outputs := runPublicKeyModule(t, privateKeyPEM)
	requirePublicKeyMatchesPrivateKey(t, key, outputs[outputPEM])
	requirePublicOpenSSHMatchesPrivateKey(t, key, outputs)
	require.True(t, strings.HasPrefix(outputs[outputOpenSSH], "ssh-rsa "))
}

// TestPublicKeySetDerivesPKCS8ECDSA verifies public key derivation from PKCS#8 ECDSA.
func TestPublicKeySetDerivesPKCS8ECDSA(t *testing.T) {
	key := testECDSAKey(t)
	privateKeyPEM := encodeTestPrivateKeyPEM(t, key)

	outputs := runPublicKeyModule(t, privateKeyPEM)
	requirePublicKeyMatchesPrivateKey(t, key, outputs[outputPEM])
	requirePublicOpenSSHMatchesPrivateKey(t, key, outputs)
	require.True(t, strings.HasPrefix(outputs[outputOpenSSH], "ecdsa-sha2-nistp256 "))
}

// TestPublicKeySetDerivesPKCS8Ed25519 verifies public key derivation from PKCS#8 Ed25519.
func TestPublicKeySetDerivesPKCS8Ed25519(t *testing.T) {
	key := testED25519Key(t)
	privateKeyPEM := encodeTestPrivateKeyPEM(t, key)

	outputs := runPublicKeyModule(t, privateKeyPEM)
	requirePublicKeyMatchesPrivateKey(t, key, outputs[outputPEM])
	requirePublicOpenSSHMatchesPrivateKey(t, key, outputs)
	require.True(t, strings.HasPrefix(outputs[outputOpenSSH], "ssh-ed25519 "))
}

// TestPublicKeySetDerivesPKCS1RSA verifies public key derivation from PKCS#1 RSA.
func TestPublicKeySetDerivesPKCS1RSA(t *testing.T) {
	key := testRSAKey(t)
	der := x509.MarshalPKCS1PrivateKey(key)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeRSAPrivateKey, Bytes: der}))

	outputs := runPublicKeyModule(t, privateKeyPEM)
	requirePublicKeyMatchesPrivateKey(t, key, outputs[outputPEM])
	requirePublicOpenSSHMatchesPrivateKey(t, key, outputs)
}

// TestPublicKeySetDerivesSEC1ECDSA verifies public key derivation from SEC1 ECDSA.
func TestPublicKeySetDerivesSEC1ECDSA(t *testing.T) {
	key := testECDSAKey(t)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeECPrivateKey, Bytes: der}))

	outputs := runPublicKeyModule(t, privateKeyPEM)
	requirePublicKeyMatchesPrivateKey(t, key, outputs[outputPEM])
	requirePublicOpenSSHMatchesPrivateKey(t, key, outputs)
}

// TestPublicKeySetRejectsInvalidPEM verifies invalid PEM inputs fail clearly.
func TestPublicKeySetRejectsInvalidPEM(t *testing.T) {
	tests := []struct {
		name          string
		privateKeyPEM string
		wantErr       string
	}{
		{
			name:          "malformed",
			privateKeyPEM: "not pem",
			wantErr:       "failed parsing private key PEM",
		},
		{
			name: "unsupported_type",
			privateKeyPEM: string(
				pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")}),
			),
			wantErr: "unsupported private key PEM type",
		},
		{
			name: "encrypted",
			privateKeyPEM: string(
				pem.EncodeToMemory(
					&pem.Block{
						Type:    pemTypeRSAPrivateKey,
						Headers: map[string]string{"Proc-Type": "4,ENCRYPTED"},
						Bytes:   []byte("invalid"),
					},
				),
			),
			wantErr: "encrypted private keys are not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := publicKeyModule{}
			ctx := blackstart.OpContext(context.Background(), publicKeyOperation(tt.privateKeyPEM))
			ok, err := m.Check(ctx)
			require.NoError(t, err)
			require.False(t, ok)
			err = m.Set(ctx)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPublicKeyInfoDocumentsAcceptedFormats verifies docs metadata lists accepted PEM formats.
func TestPublicKeyInfoDocumentsAcceptedFormats(t *testing.T) {
	info := (&publicKeyModule{}).Info()

	require.Contains(t, info.Inputs[inputPrivateKeyPEM].Description, "PKCS#8")
	require.Contains(t, info.Inputs[inputPrivateKeyPEM].Description, "PKCS#1 RSA")
	require.Contains(t, info.Inputs[inputPrivateKeyPEM].Description, "SEC1 ECDSA")
	require.Contains(t, info.Inputs[inputPrivateKeyPEM].Description, "Ed25519")
	require.Contains(t, info.Outputs, outputPEM)
	require.Contains(t, info.Outputs, outputOpenSSH)
	require.Contains(t, info.Outputs, outputMD5)
	require.Contains(t, info.Outputs, outputSHA256)
}

// runPublicKeyModule runs crypto_public_key and returns captured string outputs.
func runPublicKeyModule(t *testing.T, privateKeyPEM string) map[string]string {
	t.Helper()

	m := publicKeyModule{}
	op := publicKeyOperation(privateKeyPEM)
	require.NoError(t, m.Validate(*op))

	ctx := &capturingModuleContext{ModuleContext: blackstart.OpContext(context.Background(), op)}
	ok, err := m.Check(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, m.Set(ctx))

	return stringOutputs(t, ctx.outputs)
}

// publicKeyOperation creates a crypto_public_key test operation.
func publicKeyOperation(privateKeyPEM string) *blackstart.Operation {
	return &blackstart.Operation{
		Id:     "public",
		Module: moduleIDPublicKey,
		Inputs: map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(privateKeyPEM),
		},
	}
}

// testRSAKey creates an RSA key for tests.
func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// testECDSAKey creates an ECDSA key for tests.
func testECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

// testED25519Key creates an Ed25519 key for tests.
func testED25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return key
}

// encodeTestPrivateKeyPEM returns a PKCS#8 private key PEM for tests.
func encodeTestPrivateKeyPEM(t *testing.T, key any) string {
	t.Helper()

	privateKeyPEM, err := encodePrivateKeyPEM(key)
	require.NoError(t, err)
	return privateKeyPEM
}

// requirePublicOpenSSHMatchesPrivateKey verifies public OpenSSH outputs match a private key.
func requirePublicOpenSSHMatchesPrivateKey(t *testing.T, privateKey any, outputs map[string]string) {
	t.Helper()

	publicKey, err := publicKeyFromPrivateKey(privateKey)
	require.NoError(t, err)
	expectedOpenSSH, err := encodePublicKeyOpenSSH(publicKey)
	require.NoError(t, err)
	expectedMD5, expectedSHA256, err := publicKeyFingerprints(publicKey)
	require.NoError(t, err)
	require.Equal(t, expectedOpenSSH, outputs[outputOpenSSH])
	require.Equal(t, expectedMD5, outputs[outputMD5])
	require.Equal(t, expectedSHA256, outputs[outputSHA256])
	require.Regexp(t, `^([0-9a-f]{2}:){15}[0-9a-f]{2}$`, outputs[outputMD5])
	require.True(t, strings.HasPrefix(outputs[outputSHA256], "SHA256:"))
}
