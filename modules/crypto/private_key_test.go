package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// capturingModuleContext records module outputs while preserving normal context behavior.
type capturingModuleContext struct {
	blackstart.ModuleContext
	outputs map[string]interface{}
}

// Output records the output value and delegates to the wrapped ModuleContext.
func (c *capturingModuleContext) Output(key string, value interface{}) error {
	if c.outputs == nil {
		c.outputs = map[string]interface{}{}
	}
	c.outputs[key] = value
	return c.ModuleContext.Output(key, value)
}

// TestPrivateKeyValidate verifies private key module input validation.
func TestPrivateKeyValidate(t *testing.T) {
	m := privateKeyModule{}

	tests := []struct {
		name    string
		inputs  map[string]blackstart.Input
		wantErr string
	}{
		{
			name:    "missing_algorithm",
			inputs:  map[string]blackstart.Input{},
			wantErr: "missing required parameter: algorithm",
		},
		{
			name: "invalid_algorithm",
			inputs: map[string]blackstart.Input{
				inputAlgorithm: blackstart.NewInputFromValue("dsa"),
			},
			wantErr: "allowed values are RSA, ECDSA, ED25519",
		},
		{
			name: "valid_ed25519",
			inputs: map[string]blackstart.Input{
				inputAlgorithm: blackstart.NewInputFromValue("ed25519"),
			},
		},
		{
			name: "invalid_rsa_bits",
			inputs: map[string]blackstart.Input{
				inputAlgorithm: blackstart.NewInputFromValue("RSA"),
				inputRSABits:   blackstart.NewInputFromValue(1024),
			},
			wantErr: "allowed values are 2048, 3072, 4096",
		},
		{
			name: "valid_rsa_bits",
			inputs: map[string]blackstart.Input{
				inputAlgorithm: blackstart.NewInputFromValue("RSA"),
				inputRSABits:   blackstart.NewInputFromValue(2048),
			},
		},
		{
			name: "invalid_ecdsa_curve",
			inputs: map[string]blackstart.Input{
				inputAlgorithm:  blackstart.NewInputFromValue("ECDSA"),
				inputECDSACurve: blackstart.NewInputFromValue("P224"),
			},
			wantErr: "allowed values are P256, P384, P521",
		},
		{
			name: "valid_ecdsa_curve_alias",
			inputs: map[string]blackstart.Input{
				inputAlgorithm:  blackstart.NewInputFromValue("ECDSA"),
				inputECDSACurve: blackstart.NewInputFromValue("P-384"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Validate(blackstart.Operation{Id: "key", Module: moduleIDPrivateKey, Inputs: tt.inputs})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestPrivateKeyCheckRejectsDoesNotExist verifies doesNotExist is unsupported.
func TestPrivateKeyCheckRejectsDoesNotExist(t *testing.T) {
	m := privateKeyModule{}
	op := privateKeyOperation(map[string]blackstart.Input{
		inputAlgorithm: blackstart.NewInputFromValue("RSA"),
	})
	op.DoesNotExist = true

	ok, err := m.Check(blackstart.OpContext(context.Background(), op))
	require.False(t, ok)
	require.Error(t, err)
	require.Contains(t, err.Error(), "doesNotExist is not supported")
}

// TestPrivateKeySetGeneratesRSA verifies RSA private key output.
func TestPrivateKeySetGeneratesRSA(t *testing.T) {
	outputs := runPrivateKeyModule(t, map[string]blackstart.Input{
		inputAlgorithm: blackstart.NewInputFromValue("rsa"),
		inputRSABits:   blackstart.NewInputFromValue(2048),
	})

	privateKey := parseTestPrivateKey(t, outputs[outputPEM])
	requirePublicFingerprintsMatchPrivateKey(t, privateKey, outputs)

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	require.True(t, ok)
	require.Equal(t, 2048, rsaKey.N.BitLen())
}

// TestPrivateKeySetGeneratesECDSA verifies ECDSA private key output.
func TestPrivateKeySetGeneratesECDSA(t *testing.T) {
	outputs := runPrivateKeyModule(t, map[string]blackstart.Input{
		inputAlgorithm:  blackstart.NewInputFromValue("ECDSA"),
		inputECDSACurve: blackstart.NewInputFromValue("P-384"),
	})

	privateKey := parseTestPrivateKey(t, outputs[outputPEM])
	requirePublicFingerprintsMatchPrivateKey(t, privateKey, outputs)

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	require.True(t, ok)
	require.Equal(t, "P-384", ecdsaKey.Curve.Params().Name)
}

// TestPrivateKeySetGeneratesEd25519 verifies Ed25519 private key output.
func TestPrivateKeySetGeneratesEd25519(t *testing.T) {
	outputs := runPrivateKeyModule(t, map[string]blackstart.Input{
		inputAlgorithm: blackstart.NewInputFromValue("ED25519"),
	})

	privateKey := parseTestPrivateKey(t, outputs[outputPEM])
	requirePublicFingerprintsMatchPrivateKey(t, privateKey, outputs)

	_, ok := privateKey.(ed25519.PrivateKey)
	require.True(t, ok)
}

// TestPrivateKeySetGeneratesDifferentKeys verifies generated keys are ephemeral.
func TestPrivateKeySetGeneratesDifferentKeys(t *testing.T) {
	inputs := map[string]blackstart.Input{
		inputAlgorithm: blackstart.NewInputFromValue("RSA"),
		inputRSABits:   blackstart.NewInputFromValue(2048),
	}

	first := runPrivateKeyModule(t, inputs)
	second := runPrivateKeyModule(t, inputs)
	require.NotEqual(t, first[outputPEM], second[outputPEM])
}

// TestPrivateKeyInfoDocumentsAllowedValues verifies generated docs include value choices.
func TestPrivateKeyInfoDocumentsAllowedValues(t *testing.T) {
	info := (&privateKeyModule{}).Info()

	require.Contains(t, info.Inputs[inputAlgorithm].Description, "Allowed values: `RSA`, `ECDSA`, `ED25519`")
	require.Contains(t, info.Inputs[inputRSABits].Description, "Allowed values: `2048`, `3072`, `4096`")
	require.Contains(t, info.Inputs[inputECDSACurve].Description, "Allowed values: `P256`, `P384`, `P521`")
	require.Contains(t, info.Outputs, outputPEM)
	require.Contains(t, info.Outputs, outputMD5)
	require.Contains(t, info.Outputs, outputSHA256)
	require.NotContains(t, info.Outputs, outputOpenSSH)
}

// runPrivateKeyModule runs crypto_private_key and returns captured string outputs.
func runPrivateKeyModule(t *testing.T, inputs map[string]blackstart.Input) map[string]string {
	t.Helper()

	m := privateKeyModule{}
	op := privateKeyOperation(inputs)
	require.NoError(t, m.Validate(*op))

	ctx := &capturingModuleContext{ModuleContext: blackstart.OpContext(context.Background(), op)}
	ok, err := m.Check(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	require.NoError(t, m.Set(ctx))

	return stringOutputs(t, ctx.outputs)
}

// privateKeyOperation creates a crypto_private_key test operation.
func privateKeyOperation(inputs map[string]blackstart.Input) *blackstart.Operation {
	return &blackstart.Operation{Id: "key", Module: moduleIDPrivateKey, Inputs: inputs}
}

// parseTestPrivateKey parses a PKCS#8 private key output.
func parseTestPrivateKey(t *testing.T, value string) any {
	t.Helper()

	block, _ := pem.Decode([]byte(value))
	require.NotNil(t, block)
	require.Equal(t, pemTypePrivateKey, block.Type)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	require.NoError(t, err)
	return key
}

// requirePublicKeyMatchesPrivateKey verifies a public key PEM matches a private key.
func requirePublicKeyMatchesPrivateKey(t *testing.T, privateKey any, publicKeyPEM string) {
	t.Helper()

	expectedPublicKey, err := publicKeyFromPrivateKey(privateKey)
	require.NoError(t, err)
	expectedPEM, err := encodePublicKeyPEM(expectedPublicKey)
	require.NoError(t, err)
	require.Equal(t, expectedPEM, publicKeyPEM)
}

// requirePublicFingerprintsMatchPrivateKey verifies fingerprints match a private key.
func requirePublicFingerprintsMatchPrivateKey(t *testing.T, privateKey any, outputs map[string]string) {
	t.Helper()

	publicKey, err := publicKeyFromPrivateKey(privateKey)
	require.NoError(t, err)
	expectedMD5, expectedSHA256, err := publicKeyFingerprints(publicKey)
	require.NoError(t, err)
	require.Equal(t, expectedMD5, outputs[outputMD5])
	require.Equal(t, expectedSHA256, outputs[outputSHA256])
	require.Regexp(t, `^([0-9a-f]{2}:){15}[0-9a-f]{2}$`, outputs[outputMD5])
	require.True(t, strings.HasPrefix(outputs[outputSHA256], "SHA256:"))
}

// stringOutputs converts captured outputs to strings.
func stringOutputs(t *testing.T, outputs map[string]interface{}) map[string]string {
	t.Helper()

	out := make(map[string]string, len(outputs))
	for key, value := range outputs {
		stringValue, ok := value.(string)
		require.True(t, ok, "output %s should be string", key)
		out[key] = stringValue
	}
	return out
}
