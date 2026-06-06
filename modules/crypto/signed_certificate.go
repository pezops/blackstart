package crypto

import (
	"crypto/rand"
	stdx509 "crypto/x509"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

// NewSignedCertificate creates a module that signs a CSR using a local CA certificate and key.
func NewSignedCertificate() blackstart.Module {
	return &signedCertificateModule{}
}

// signedCertificateTarget contains normalized CSR signing inputs.
type signedCertificateTarget struct {
	CSRPEM           string
	CACertificatePEM string
	CAPrivateKeyPEM  string
	CAChainPEM       string
	Profile          string
	ValidityHours    int64
}

// signedCertificateModule signs certificate signing requests.
type signedCertificateModule struct {
	target *signedCertificateTarget
}

// Info returns metadata describing the signed certificate module.
func (m *signedCertificateModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   moduleIDSignedCert,
		Name: "X.509 signed certificate",
		Description: util.CleanString(
			`
Signs a PEM-encoded X.509 certificate signing request using a local CA certificate and private key.
`,
		),
		Requirements: []string{
			"The CSR must be PEM encoded and have a valid signature.",
			"The CA certificate must be a PEM-encoded CA certificate.",
			"The CA private key must be unencrypted and PEM encoded.",
			"This module does not assert public-trust CA compliance.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputCSRPEM: {
				Description: "PEM-encoded X.509 certificate signing request.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputCACertificatePEM: {
				Description: "PEM-encoded CA certificate used as the issuer.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputCAPrivateKeyPEM: {
				Description: "PEM-encoded CA private key used to sign the certificate. Encrypted private keys are not supported.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputCAChainPEM: {
				Description: "Optional PEM-encoded certificates to append after the signing CA in chain outputs.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputProfile: {
				Description: "TLS certificate profile for the issued certificate. Allowed values: `server`, `client`, `server_client`, `ca`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     profileServer,
			},
			inputValidityHours: {
				Description: fmt.Sprintf(
					"Certificate validity period in hours. Defaults to 46 days (1104 hours), aligned with the future CA/Browser Forum 46-day recommendation in section 6.3.2: %s",
					cabForumValiditySectionURL,
				),
				Type:     reflect.TypeFor[int](),
				Required: false,
				Default:  defaultValidityHours,
			},
		},
		Outputs: certificateOutputInfo(true),
		Examples: map[string]string{
			"Sign server CSR with local CA": `
operations:
  - id: ca_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: ca_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: ca_key
          output: pem
      profile: ca
      common_name: Example Internal CA

  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_csr
    module: crypto_x509_certificate_request
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      common_name: app.example.com
      dns_names:
        - app.example.com

  - id: server_cert
    module: crypto_x509_signed_certificate
    inputs:
      csr_pem:
        fromDependency:
          id: server_csr
          output: pem
      ca_certificate_pem:
        fromDependency:
          id: ca_cert
          output: pem
      ca_private_key_pem:
        fromDependency:
          id: ca_key
          output: pem
      profile: server`,
		},
	}
}

// Validate checks whether an operation contains valid signed certificate inputs.
func (m *signedCertificateModule) Validate(op blackstart.Operation) error {
	if err := validateRequiredStaticCSR(op, inputCSRPEM); err != nil {
		return err
	}
	if err := validateRequiredStaticCertificate(op, inputCACertificatePEM); err != nil {
		return err
	}
	if err := validateRequiredStaticPrivateKey(op, inputCAPrivateKeyPEM); err != nil {
		return err
	}
	if err := validateStaticCertificateChain(op, inputCAChainPEM); err != nil {
		return err
	}
	if err := validateStaticProfile(op); err != nil {
		return err
	}
	return validateStaticValidityHours(op)
}

// Check creates the signed certificate target and always returns false.
func (m *signedCertificateModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", moduleIDSignedCert)
	}
	if err := m.createTarget(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set signs the CSR and emits certificate outputs.
func (m *signedCertificateModule) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", moduleIDSignedCert)
	}
	if m.target == nil {
		if err := m.createTarget(ctx); err != nil {
			return err
		}
	}

	csr, err := parseCSRPEM(m.target.CSRPEM)
	if err != nil {
		return err
	}
	caCert, err := parseCertificatePEM(m.target.CACertificatePEM)
	if err != nil {
		return err
	}
	if !caCert.IsCA {
		return fmt.Errorf("ca_certificate_pem must be a CA certificate")
	}
	caKey, err := parsePrivateKeyPEM(m.target.CAPrivateKeyPEM)
	if err != nil {
		return err
	}
	if err = validateCAPrivateKeyMatchesCertificate(caKey, caCert); err != nil {
		return err
	}
	if strings.TrimSpace(m.target.CAChainPEM) != "" {
		if _, err = parseCertificateChainPEM(m.target.CAChainPEM); err != nil {
			return err
		}
	}

	identity := certIdentity{
		Subject: certSubject{
			CommonName:         csr.Subject.CommonName,
			Organization:       csr.Subject.Organization,
			OrganizationalUnit: csr.Subject.OrganizationalUnit,
			Country:            csr.Subject.Country,
			Locality:           csr.Subject.Locality,
			Province:           csr.Subject.Province,
		},
		SANs: certSANs{
			DNSNames:       csr.DNSNames,
			IPAddresses:    csr.IPAddresses,
			EmailAddresses: csr.EmailAddresses,
			URIs:           csr.URIs,
		},
	}
	template, err := certificateTemplate(m.target.Profile, identity, time.Now(), m.target.ValidityHours)
	if err != nil {
		return err
	}
	template.PublicKey = csr.PublicKey

	der, err := stdx509.CreateCertificate(rand.Reader, template, caCert, csr.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("failed creating signed certificate: %w", err)
	}
	cert, err := stdx509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("failed parsing generated certificate: %w", err)
	}

	chainPEM := m.target.CACertificatePEM + m.target.CAChainPEM
	outputs := certificateOutputs(cert, der, chainPEM)
	return outputCertificate(ctx, outputs, true)
}

// createTarget reads signed certificate inputs from the module context.
func (m *signedCertificateModule) createTarget(ctx blackstart.ModuleContext) error {
	csrPEM, err := blackstart.ContextInputAs[string](ctx, inputCSRPEM, true)
	if err != nil {
		return err
	}
	caCertificatePEM, err := blackstart.ContextInputAs[string](ctx, inputCACertificatePEM, true)
	if err != nil {
		return err
	}
	caPrivateKeyPEM, err := blackstart.ContextInputAs[string](ctx, inputCAPrivateKeyPEM, true)
	if err != nil {
		return err
	}
	caChainPEM, err := blackstart.ContextInputAs[string](ctx, inputCAChainPEM, false)
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
	validityHours, err := readValidityHours(ctx)
	if err != nil {
		return err
	}

	m.target = &signedCertificateTarget{
		CSRPEM:           csrPEM,
		CACertificatePEM: caCertificatePEM,
		CAPrivateKeyPEM:  caPrivateKeyPEM,
		CAChainPEM:       caChainPEM,
		Profile:          profile,
		ValidityHours:    validityHours,
	}
	return nil
}
