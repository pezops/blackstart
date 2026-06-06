package crypto

import (
	"crypto/rand"
	stdx509 "crypto/x509"
	"fmt"
	"reflect"
	"time"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

// NewSelfSignedCertificate creates a module that generates a self-signed X.509 certificate.
func NewSelfSignedCertificate() blackstart.Module {
	return &selfSignedCertificateModule{}
}

// selfSignedCertificateTarget contains normalized self-signed certificate inputs.
type selfSignedCertificateTarget struct {
	PrivateKeyPEM string
	Profile       string
	Identity      certIdentity
	ValidityHours int64
}

// selfSignedCertificateModule generates self-signed certificates.
type selfSignedCertificateModule struct {
	target *selfSignedCertificateTarget
}

// Info returns metadata describing the self-signed certificate module.
func (m *selfSignedCertificateModule) Info() blackstart.ModuleInfo {
	inputs := mergeInputs(subjectInputs(), sanInputs())
	inputs[inputPrivateKeyPEM] = blackstart.InputValue{
		Description: "PEM-encoded private key. Accepted formats: PKCS#8, PKCS#1 RSA, or SEC1 ECDSA. Encrypted private keys are not supported.",
		Type:        reflect.TypeFor[string](),
		Required:    true,
	}
	inputs[inputProfile] = blackstart.InputValue{
		Description: "TLS certificate profile. Allowed values: `server`, `client`, `server_client`, `ca`.",
		Type:        reflect.TypeFor[string](),
		Required:    false,
		Default:     profileServer,
	}
	inputs[inputValidityHours] = blackstart.InputValue{
		Description: fmt.Sprintf(
			"Certificate validity period in hours. Defaults to 46 days (1104 hours), aligned with the future CA/Browser Forum 46-day recommendation in section 6.3.2: %s",
			cabForumValiditySectionURL,
		),
		Type:     reflect.TypeFor[int](),
		Required: false,
		Default:  defaultValidityHours,
	}

	return blackstart.ModuleInfo{
		Id:   moduleIDSelfSignedCert,
		Name: "X.509 self-signed certificate",
		Description: util.CleanString(
			`
Generates a PEM-encoded self-signed X.509 certificate.
`,
		),
		Requirements: []string{
			"The private key must be unencrypted and PEM encoded.",
			"TLS identities should be provided through subject alternative name inputs such as `dns_names` or `ip_addresses`.",
			"This module does not assert public-trust CA compliance.",
		},
		Inputs:  inputs,
		Outputs: certificateOutputInfo(false),
		Examples: map[string]string{
			"Generate self-signed server certificate": `
operations:
  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      profile: server
      common_name: app.example.com
      dns_names:
        - app.example.com`,
			"Store self-signed certificate in a Kubernetes TLS Secret": `
operations:
  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      profile: server
      common_name: app.example.com
      dns_names:
        - app.example.com

  - id: k8s_client
    module: kubernetes_client

  - id: tls_secret
    module: kubernetes_secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: default
      name: app-tls
      type: kubernetes.io/tls

  - id: tls_crt
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: tls_secret
          output: secret
      key: tls.crt
      value:
        fromDependency:
          id: server_cert
          output: pem
      update_policy: overwrite

  - id: tls_key
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: tls_secret
          output: secret
      key: tls.key
      value:
        fromDependency:
          id: server_key
          output: pem
      update_policy: overwrite`,
		},
	}
}

// Validate checks whether an operation contains valid self-signed certificate inputs.
func (m *selfSignedCertificateModule) Validate(op blackstart.Operation) error {
	if err := validateRequiredStaticPrivateKey(op, inputPrivateKeyPEM); err != nil {
		return err
	}
	if err := validateStaticProfile(op); err != nil {
		return err
	}
	if err := validateStaticValidityHours(op); err != nil {
		return err
	}
	return validateStaticSANs(op)
}

// Check creates the certificate target and always returns false.
func (m *selfSignedCertificateModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDSelfSignedCert)
	}
	if err := m.createTarget(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set generates and emits the self-signed certificate.
func (m *selfSignedCertificateModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDSelfSignedCert)
	}
	if m.target == nil {
		if err := m.createTarget(ctx); err != nil {
			return err
		}
	}

	key, err := parsePrivateKeyPEM(m.target.PrivateKeyPEM)
	if err != nil {
		return err
	}
	publicKey, err := publicKeyFromPrivateKey(key)
	if err != nil {
		return err
	}
	template, err := certificateTemplate(
		m.target.Profile, m.target.Identity, time.Now(), m.target.ValidityHours,
	)
	if err != nil {
		return err
	}
	der, err := stdx509.CreateCertificate(rand.Reader, template, template, publicKey, key)
	if err != nil {
		return fmt.Errorf("failed creating self-signed certificate: %w", err)
	}
	cert, err := stdx509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("failed parsing generated certificate: %w", err)
	}

	outputs := certificateOutputs(cert, der, "")
	return outputCertificate(ctx, outputs, false)
}

// createTarget reads self-signed certificate inputs from the module context.
func (m *selfSignedCertificateModule) createTarget(ctx blackstart.ModuleContext) error {
	privateKeyPEM, err := blackstart.ContextInputAs[string](ctx, inputPrivateKeyPEM, true)
	if err != nil {
		return err
	}
	profileInput, err := blackstart.ContextInputAs[string](ctx, inputProfile, false)
	if err != nil {
		return err
	}
	if profileInput == "" {
		profileInput = profileServer
	}
	profile, err := normalizeProfile(profileInput)
	if err != nil {
		return err
	}
	identity, err := readIdentity(ctx)
	if err != nil {
		return err
	}
	validityHours, err := readValidityHours(ctx)
	if err != nil {
		return err
	}

	m.target = &selfSignedCertificateTarget{
		PrivateKeyPEM: privateKeyPEM,
		Profile:       profile,
		Identity:      identity,
		ValidityHours: validityHours,
	}
	return nil
}
