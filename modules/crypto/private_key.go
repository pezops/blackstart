package crypto

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

const moduleIDPrivateKey = "crypto_private_key"

func init() {
	blackstart.RegisterModule(moduleIDPrivateKey, NewPrivateKey)
}

// NewPrivateKey creates a module that generates an ephemeral private key.
func NewPrivateKey() blackstart.Module {
	return &privateKeyModule{}
}

// privateKeyTarget contains normalized private key generation settings.
type privateKeyTarget struct {
	// algorithm is the normalized key algorithm.
	algorithm string
	// rsaBits is the RSA key size when algorithm is RSA.
	rsaBits int
	// ecdsaCurve is the normalized ECDSA curve name when algorithm is ECDSA.
	ecdsaCurve string
}

// privateKeyModule generates RSA, ECDSA, and Ed25519 private keys.
type privateKeyModule struct {
	target *privateKeyTarget
}

// Info returns metadata describing the crypto private key module.
func (m *privateKeyModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   moduleIDPrivateKey,
		Name: "Crypto private key",
		Description: util.CleanString(
			`
Generates a cryptographic private key.

Generated private keys are ephemeral. The output must be persisted in a storage operation if it is 
needed after the workflow completes.
`,
		),
		Inputs: map[string]blackstart.InputValue{
			inputAlgorithm: {
				Description: "Private key algorithm. Allowed values: `RSA`, `ECDSA`, `ED25519`.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputRSABits: {
				Description: "RSA key size in bits. Allowed values: `2048`, `3072`, `4096`. Only used when `algorithm` is `RSA`.",
				Type:        reflect.TypeFor[int](),
				Required:    false,
				Default:     4096,
			},
			inputECDSACurve: {
				Description: "ECDSA curve. Allowed values: `P256`, `P384`, `P521`. Aliases `P-256`, `P-384`, and `P-521` are accepted. Only used when `algorithm` is `ECDSA`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "P256",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputPEM: {
				Description: "PEM-encoded private key in PKCS#8 format.",
				Type:        reflect.TypeFor[string](),
			},
			outputMD5: {
				Description: "OpenSSH MD5 fingerprint derived from the generated public key.",
				Type:        reflect.TypeFor[string](),
			},
			outputSHA256: {
				Description: "OpenSSH SHA256 fingerprint derived from the generated public key.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Generate RSA private key": `id: generate-rsa-key
module: crypto_private_key
inputs:
  algorithm: RSA
  rsa_bits: 4096`,
			"Generate ECDSA private key": `id: generate-ecdsa-key
module: crypto_private_key
inputs:
  algorithm: ECDSA
  ecdsa_curve: P256`,
			"Generate Ed25519 private key": `id: generate-ed25519-key
module: crypto_private_key
inputs:
  algorithm: ED25519`,
		},
	}
}

// Validate checks whether an operation contains valid private key inputs.
func (m *privateKeyModule) Validate(op blackstart.Operation) error {
	algorithmInput, ok := op.Inputs[inputAlgorithm]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", inputAlgorithm)
	}
	if algorithmInput.IsStatic() {
		algorithm, err := blackstart.InputAs[string](algorithmInput, true)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputAlgorithm, err)
		}
		if _, err = normalizeAlgorithm(algorithm); err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputAlgorithm, err)
		}
	}

	if err := validateStaticRSABits(op); err != nil {
		return err
	}
	if err := validateStaticECDSACurve(op); err != nil {
		return err
	}
	return nil
}

// Check creates the target key settings and always returns false.
func (m *privateKeyModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDPrivateKey)
	}
	if err := m.createTarget(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set generates the private key and emits private key outputs.
func (m *privateKeyModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDPrivateKey)
	}

	if m.target == nil {
		if err := m.createTarget(ctx); err != nil {
			return err
		}
	}

	key, err := generatePrivateKey(m.target)
	if err != nil {
		return err
	}

	privateKeyPEM, err := encodePrivateKeyPEM(key)
	if err != nil {
		return err
	}

	publicKey, err := publicKeyFromPrivateKey(key)
	if err != nil {
		return err
	}

	fpMD5, fpSHA256, err := publicKeyFingerprints(publicKey)
	if err != nil {
		return err
	}

	if err = ctx.Output(outputPEM, privateKeyPEM); err != nil {
		return err
	}

	if err = ctx.Output(outputMD5, fpMD5); err != nil {
		return err
	}
	return ctx.Output(outputSHA256, fpSHA256)
}

// createTarget creates normalized private key settings from context inputs.
func (m *privateKeyModule) createTarget(ctx blackstart.ModuleContext) error {
	algorithmInput, err := blackstart.ContextInputAs[string](ctx, inputAlgorithm, true)
	if err != nil {
		return err
	}
	algorithm, err := normalizeAlgorithm(algorithmInput)
	if err != nil {
		return err
	}
	rsaBitsInput, err := blackstart.ContextInputAs[int64](ctx, inputRSABits, true)
	if err != nil {
		return err
	}
	rsaBits, err := normalizeRSABits(rsaBitsInput)
	if err != nil {
		return err
	}
	ecdsaCurveInput, err := blackstart.ContextInputAs[string](ctx, inputECDSACurve, true)
	if err != nil {
		return err
	}
	ecdsaCurve, err := normalizeECDSACurve(ecdsaCurveInput)
	if err != nil {
		return err
	}

	m.target = &privateKeyTarget{
		algorithm:  algorithm,
		rsaBits:    rsaBits,
		ecdsaCurve: ecdsaCurve,
	}
	return nil
}

// generatePrivateKey creates a private key for the target settings.
func generatePrivateKey(target *privateKeyTarget) (any, error) {
	switch target.algorithm {
	case algorithmRSA:
		return rsa.GenerateKey(rand.Reader, target.rsaBits)
	case algorithmECDSA:
		curve, err := ellipticCurve(target.ecdsaCurve)
		if err != nil {
			return nil, err
		}
		return ecdsa.GenerateKey(curve, rand.Reader)
	case algorithmED25519:
		_, key, err := ed25519.GenerateKey(rand.Reader)
		return key, err
	default:
		return nil, fmt.Errorf("unsupported private key algorithm %q", target.algorithm)
	}
}

// ellipticCurve returns the standard library curve for a normalized curve name.
func ellipticCurve(name string) (elliptic.Curve, error) {
	switch name {
	case "P256":
		return elliptic.P256(), nil
	case "P384":
		return elliptic.P384(), nil
	case "P521":
		return elliptic.P521(), nil
	default:
		return nil, fmt.Errorf("unsupported ECDSA curve %q", name)
	}
}

// validateStaticRSABits validates a static rsa_bits input when configured.
func validateStaticRSABits(op blackstart.Operation) error {
	input, ok := op.Inputs[inputRSABits]
	if !ok || !input.IsStatic() {
		return nil
	}
	bits, err := blackstart.InputAs[int64](input, false)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputRSABits, err)
	}
	if _, err = normalizeRSABits(bits); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputRSABits, err)
	}
	return nil
}

// validateStaticECDSACurve validates a static ecdsa_curve input when configured.
func validateStaticECDSACurve(op blackstart.Operation) error {
	input, ok := op.Inputs[inputECDSACurve]
	if !ok || !input.IsStatic() {
		return nil
	}
	curve, err := blackstart.InputAs[string](input, false)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputECDSACurve, err)
	}
	if _, err = normalizeECDSACurve(curve); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputECDSACurve, err)
	}
	return nil
}
