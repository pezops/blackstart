package crypto

import (
	"crypto/rand"
	stdx509 "crypto/x509"
	"fmt"
	"reflect"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

// NewCertificateRequest creates a module that generates an X.509 CSR.
func NewCertificateRequest() blackstart.Module {
	return &certificateRequestModule{}
}

// certificateRequestTarget contains normalized CSR generation inputs.
type certificateRequestTarget struct {
	PrivateKeyPEM string
	Identity      certIdentity
}

// certificateRequestModule generates certificate signing requests.
type certificateRequestModule struct {
	target *certificateRequestTarget
}

// Info returns metadata describing the X.509 certificate request module.
func (m *certificateRequestModule) Info() blackstart.ModuleInfo {
	inputs := mergeInputs(subjectInputs(), sanInputs())
	inputs[inputPrivateKeyPEM] = blackstart.InputValue{
		Description: "PEM-encoded private key. Accepted formats: PKCS#8, PKCS#1 RSA, or SEC1 ECDSA. Encrypted private keys are not supported.",
		Type:        reflect.TypeFor[string](),
		Required:    true,
	}

	return blackstart.ModuleInfo{
		Id:   moduleIDCertificateRequest,
		Name: "X.509 certificate request",
		Description: util.CleanString(
			`
Generates a PEM-encoded X.509 certificate signing request (CSR).
`,
		),
		Requirements: []string{
			"The private key must be unencrypted and PEM encoded.",
			"TLS identities should be provided through subject alternative name inputs such as `dns_names` or `ip_addresses`.",
		},
		Inputs: inputs,
		Outputs: map[string]blackstart.OutputValue{
			outputPEM: {
				Description: "PEM-encoded X.509 certificate signing request.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Generate server CSR": `
id: server-csr
module: crypto_x509_certificate_request
inputs:
  private_key_pem:
    fromDependency:
      id: server-key
      output: pem
  common_name: app.example.com
  dns_names:
    - app.example.com
    - www.app.example.com`,
		},
	}
}

// Validate checks whether an operation contains valid CSR inputs.
func (m *certificateRequestModule) Validate(op blackstart.Operation) error {
	if err := validateRequiredStaticPrivateKey(op, inputPrivateKeyPEM); err != nil {
		return err
	}
	return validateStaticSANs(op)
}

// Check creates the CSR target and always returns false.
func (m *certificateRequestModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDCertificateRequest)
	}
	if err := m.createTarget(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set generates and emits the certificate signing request.
func (m *certificateRequestModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDCertificateRequest)
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
	template := &stdx509.CertificateRequest{
		Subject:        pkixName(m.target.Identity.Subject),
		DNSNames:       m.target.Identity.SANs.DNSNames,
		IPAddresses:    m.target.Identity.SANs.IPAddresses,
		EmailAddresses: m.target.Identity.SANs.EmailAddresses,
		URIs:           m.target.Identity.SANs.URIs,
	}
	der, err := stdx509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		return fmt.Errorf("failed creating certificate request: %w", err)
	}
	return ctx.Output(outputPEM, encodeCSRPEM(der))
}

// createTarget reads CSR inputs from the module context.
func (m *certificateRequestModule) createTarget(ctx blackstart.ModuleContext) error {
	privateKeyPEM, err := blackstart.ContextInputAs[string](ctx, inputPrivateKeyPEM, true)
	if err != nil {
		return err
	}
	identity, err := readIdentity(ctx)
	if err != nil {
		return err
	}
	m.target = &certificateRequestTarget{PrivateKeyPEM: privateKeyPEM, Identity: identity}
	return nil
}
