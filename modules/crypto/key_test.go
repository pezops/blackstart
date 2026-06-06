package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNormalizeAlgorithm verifies supported algorithm values are canonicalized.
func TestNormalizeAlgorithm(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "rsa_upper", value: "RSA", want: algorithmRSA},
		{name: "rsa_lower", value: "rsa", want: algorithmRSA},
		{name: "ecdsa_spaced", value: " ecdsa ", want: algorithmECDSA},
		{name: "ed25519_lower", value: "ed25519", want: algorithmED25519},
		{name: "invalid", value: "dsa", wantErr: "allowed values are RSA, ECDSA, ED25519"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAlgorithm(tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestNormalizeRSABits verifies supported RSA bit sizes are accepted.
func TestNormalizeRSABits(t *testing.T) {
	tests := []struct {
		name    string
		value   int64
		want    int
		wantErr string
	}{
		{name: "2048", value: 2048, want: 2048},
		{name: "3072", value: 3072, want: 3072},
		{name: "4096", value: 4096, want: 4096},
		{name: "invalid", value: 1024, wantErr: "allowed values are 2048, 3072, 4096"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRSABits(tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestNormalizeECDSACurve verifies supported ECDSA curve names are canonicalized.
func TestNormalizeECDSACurve(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "p256", value: "P256", want: "P256"},
		{name: "p384_alias", value: " p-384 ", want: "P384"},
		{name: "p521_alias", value: "P-521", want: "P521"},
		{name: "invalid", value: "P224", wantErr: "allowed values are P256, P384, P521"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeECDSACurve(tt.value)
			if tt.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestEncodePrivateKeyPEM verifies private keys are encoded as PKCS#8 PEM.
func TestEncodePrivateKeyPEM(t *testing.T) {
	key := testRSAKey(t)

	privateKeyPEM, err := encodePrivateKeyPEM(key)
	require.NoError(t, err)
	block, _ := pem.Decode([]byte(privateKeyPEM))
	require.NotNil(t, block)
	require.Equal(t, pemTypePrivateKey, block.Type)
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	require.IsType(t, key, parsed)
}

// TestEncodePublicKeyPEM verifies public keys are encoded as SubjectPublicKeyInfo PEM.
func TestEncodePublicKeyPEM(t *testing.T) {
	key := testRSAKey(t)

	publicKeyPEM, err := encodePublicKeyPEM(&key.PublicKey)
	require.NoError(t, err)
	block, _ := pem.Decode([]byte(publicKeyPEM))
	require.NotNil(t, block)
	require.Equal(t, pemTypePublicKey, block.Type)
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	require.NoError(t, err)
	require.IsType(t, &key.PublicKey, parsed)
}

// TestEncodePublicKeyOpenSSH verifies public keys are encoded for authorized_keys.
func TestEncodePublicKeyOpenSSH(t *testing.T) {
	key := testRSAKey(t)

	publicKeyOpenSSH, err := encodePublicKeyOpenSSH(&key.PublicKey)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(publicKeyOpenSSH, "ssh-rsa "))
	require.True(t, strings.HasSuffix(publicKeyOpenSSH, "\n"))
}

// TestPublicKeyFingerprints verifies fingerprint output formats.
func TestPublicKeyFingerprints(t *testing.T) {
	key := testECDSAKey(t)

	fingerprintMD5, fingerprintSHA256, err := publicKeyFingerprints(&key.PublicKey)
	require.NoError(t, err)
	require.Regexp(t, `^([0-9a-f]{2}:){15}[0-9a-f]{2}$`, fingerprintMD5)
	require.True(t, strings.HasPrefix(fingerprintSHA256, "SHA256:"))
}

// TestParsePrivateKeyPEM verifies all accepted private key PEM formats parse.
func TestParsePrivateKeyPEM(t *testing.T) {
	rsaKey := testRSAKey(t)
	ecdsaKey := testECDSAKey(t)
	ed25519Key := testED25519Key(t)
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeRSAPrivateKey, Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}))
	sec1DER, err := x509.MarshalECPrivateKey(ecdsaKey)
	require.NoError(t, err)
	sec1PEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeECPrivateKey, Bytes: sec1DER}))

	tests := []struct {
		name string
		pem  string
		want any
	}{
		{name: "pkcs8_rsa", pem: encodeTestPrivateKeyPEM(t, rsaKey), want: rsaKey},
		{name: "pkcs1_rsa", pem: pkcs1PEM, want: rsaKey},
		{name: "sec1_ecdsa", pem: sec1PEM, want: ecdsaKey},
		{name: "pkcs8_ed25519", pem: encodeTestPrivateKeyPEM(t, ed25519Key), want: ed25519Key},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePrivateKeyPEM(tt.pem)
			require.NoError(t, err)
			require.IsType(t, tt.want, got)
		})
	}
}

// TestParsePrivateKeyPEMRejectsInvalidInputs verifies parse errors are clear.
func TestParsePrivateKeyPEMRejectsInvalidInputs(t *testing.T) {
	rsaKey := testRSAKey(t)
	privateKeyPEM := encodeTestPrivateKeyPEM(t, rsaKey)

	tests := []struct {
		name    string
		pem     string
		wantErr string
	}{
		{name: "malformed", pem: "not pem", wantErr: "failed parsing private key PEM"},
		{name: "multiple_blocks", pem: privateKeyPEM + privateKeyPEM, wantErr: "must contain exactly one PEM block"},
		{
			name: "encrypted_proc_type",
			pem: string(
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
		{
			name: "encrypted_dek_info",
			pem: string(
				pem.EncodeToMemory(
					&pem.Block{
						Type:    pemTypeRSAPrivateKey,
						Headers: map[string]string{"DEK-Info": "AES-256-CBC,0123456789ABCDEF"},
						Bytes:   []byte("invalid"),
					},
				),
			),
			wantErr: "encrypted private keys are not supported",
		},
		{
			name:    "unsupported_type",
			pem:     string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")})),
			wantErr: "unsupported private key PEM type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePrivateKeyPEM(tt.pem)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestSupportedPrivateKeyRejectsUnsupportedKey verifies unsupported key types fail.
func TestSupportedPrivateKeyRejectsUnsupportedKey(t *testing.T) {
	_, err := supportedPrivateKey("not-a-private-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported private key type")
}

// TestPublicKeyFromPrivateKeyRejectsUnsupportedKey verifies unsupported key types fail.
func TestPublicKeyFromPrivateKeyRejectsUnsupportedKey(t *testing.T) {
	_, err := publicKeyFromPrivateKey("not-a-private-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported private key type")
}
