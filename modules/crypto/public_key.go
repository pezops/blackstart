package crypto

import (
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

const moduleIDPublicKey = "crypto_public_key"

func init() {
	blackstart.RegisterModule(moduleIDPublicKey, NewPublicKey)
}

// NewPublicKey creates a module that derives a public key from a private key.
func NewPublicKey() blackstart.Module {
	return &publicKeyModule{}
}

// publicKeyModule derives public keys from private key PEM input.
type publicKeyModule struct {
	privateKeyPEM string
}

// Info returns metadata describing the crypto public key module.
func (m *publicKeyModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   moduleIDPublicKey,
		Name: "Crypto public key",
		Description: util.CleanString(
			`
Derives the public key from a supported private key.
`,
		),
		Requirements: []string{
			"Private key must be a single, PEM-encoded block and in a PKCS#8, PKCS#1 RSA, or SEC1 ECDSA format.",
			"Passphrase-protected private keys are not supported - the `private_key_pem` input must be unencrypted.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputPrivateKeyPEM: {
				Description: "Private key PEM. Accepted formats: PKCS#8 private key, PKCS#1 RSA private key, SEC1 ECDSA private key. Supported PKCS#8 algorithms: RSA, ECDSA, Ed25519.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputPEM: {
				Description: "PEM-encoded SubjectPublicKeyInfo (SPKI) public key.",
				Type:        reflect.TypeFor[string](),
			},
			outputOpenSSH: {
				Description: "OpenSSH authorized-key public key.",
				Type:        reflect.TypeFor[string](),
			},
			outputMD5: {
				Description: "OpenSSH MD5 public key fingerprint.",
				Type:        reflect.TypeFor[string](),
			},
			outputSHA256: {
				Description: "OpenSSH SHA256 public key fingerprint.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Derive public key from dependency": `
id: derive-public-key
module: crypto_public_key
inputs:
  private_key_pem:
    fromDependency:
      id: generate-private-key
      output: pem`,
			"Key rotation with old and new Kubernetes Secret values": `
operations:
  - id: k8s_client
    module: kubernetes_client

  - id: signing_key_secret
    module: kubernetes_secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: default
      name: signing-key

  - id: generate_new_private_key
    module: crypto_private_key
    inputs:
      algorithm: RSA
      rsa_bits: 4096

  - id: read_old_private_key
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_private_key
      value: ""
      update_policy: preserve

  - id: derive_old_public_key
    module: crypto_public_key
    inputs:
      private_key_pem:
        fromDependency:
          id: read_old_private_key
          output: value

  - id: old_private_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: old_private_key
      value:
        fromDependency:
          id: read_old_private_key
          output: value
      update_policy: overwrite

  - id: old_public_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: old_public_key
      value:
        fromDependency:
          id: derive_old_public_key
          output: openssh
      update_policy: overwrite

  - id: derive_new_public_key
    module: crypto_public_key
    inputs:
      private_key_pem:
        fromDependency:
          id: generate_new_private_key
          output: pem

  - id: new_private_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_private_key
      value:
        fromDependency:
          id: generate_new_private_key
          output: pem
      update_policy: overwrite
    dependsOn:
      - old_private_key_secret_value
      - old_public_key_secret_value

  - id: new_public_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_public_key
      value:
        fromDependency:
          id: derive_new_public_key
          output: openssh
      update_policy: overwrite
    dependsOn:
      - old_private_key_secret_value
      - old_public_key_secret_value`,
		},
	}
}

// Validate checks whether an operation contains valid public key inputs.
func (m *publicKeyModule) Validate(op blackstart.Operation) error {
	input, ok := op.Inputs[inputPrivateKeyPEM]
	if !ok {
		return fmt.Errorf("missing required parameter: %s", inputPrivateKeyPEM)
	}
	if !input.IsStatic() {
		return nil
	}
	privateKeyPEM, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputPrivateKeyPEM, err)
	}
	if _, err = parsePrivateKeyPEM(privateKeyPEM); err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputPrivateKeyPEM, err)
	}
	return nil
}

// Check creates the private key target and always returns false.
func (m *publicKeyModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDPublicKey)
	}
	if err := m.createTarget(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set derives and emits a public key from the private key input.
func (m *publicKeyModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDPublicKey)
	}
	if m.privateKeyPEM == "" {
		if err := m.createTarget(ctx); err != nil {
			return err
		}
	}

	key, err := parsePrivateKeyPEM(m.privateKeyPEM)
	if err != nil {
		return err
	}
	publicKey, err := publicKeyFromPrivateKey(key)
	if err != nil {
		return err
	}
	publicKeyPEM, err := encodePublicKeyPEM(publicKey)
	if err != nil {
		return err
	}
	publicKeyOpenSSH, err := encodePublicKeyOpenSSH(publicKey)
	if err != nil {
		return err
	}
	fingerprintMD5, fingerprintSHA256, err := publicKeyFingerprints(publicKey)
	if err != nil {
		return err
	}

	if err := ctx.Output(outputPEM, publicKeyPEM); err != nil {
		return err
	}
	if err := ctx.Output(outputOpenSSH, publicKeyOpenSSH); err != nil {
		return err
	}
	if err := ctx.Output(outputMD5, fingerprintMD5); err != nil {
		return err
	}
	return ctx.Output(outputSHA256, fingerprintSHA256)
}

// createTarget reads private key input from the module context.
func (m *publicKeyModule) createTarget(ctx blackstart.ModuleContext) error {
	privateKeyPEM, err := blackstart.ContextInputAs[string](ctx, inputPrivateKeyPEM, true)
	if err != nil {
		return err
	}
	m.privateKeyPEM = privateKeyPEM
	return nil
}
