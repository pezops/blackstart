package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/pezops/blackstart"
)

// certSubject contains optional X.509 subject fields.
type certSubject struct {
	CommonName         string
	Organization       []string
	OrganizationalUnit []string
	Country            []string
	Locality           []string
	Province           []string
}

// certSANs contains normalized certificate subject alternative names.
type certSANs struct {
	DNSNames       []string
	IPAddresses    []net.IP
	EmailAddresses []string
	URIs           []*url.URL
}

// certIdentity contains normalized certificate subject and SAN inputs.
type certIdentity struct {
	Subject certSubject
	SANs    certSANs
}

// certOutputs contains generated certificate output values.
type certOutputs struct {
	PEM          string
	ChainPEM     string
	CombinedPEM  string
	SerialNumber string
	NotBefore    string
	NotAfter     string
	SHA256       string
}

// subjectInputs returns shared subject input metadata.
func subjectInputs() map[string]blackstart.InputValue {
	return map[string]blackstart.InputValue{
		inputCommonName: {
			Description: "Certificate subject common name. For TLS certificates, SANs should carry DNS names or IP addresses.",
			Type:        reflect.TypeFor[string](),
			Required:    false,
		},
		inputOrganization: {
			Description: "Certificate subject organization value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputOrganizationalU: {
			Description: "Certificate subject organizational unit value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputCountry: {
			Description: "Certificate subject country value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputLocality: {
			Description: "Certificate subject locality value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputProvince: {
			Description: "Certificate subject state or province value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
	}
}

// sanInputs returns shared subject alternative name input metadata.
func sanInputs() map[string]blackstart.InputValue {
	return map[string]blackstart.InputValue{
		inputDNSNames: {
			Description: "DNS subject alternative name value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputIPAddresses: {
			Description: "IP address subject alternative name value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputEmailAddresses: {
			Description: "Email address subject alternative name value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
		inputURIs: {
			Description: "URI subject alternative name value or values.",
			Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
			Required:    false,
		},
	}
}

// mergeInputs returns a combined input metadata map.
func mergeInputs(maps ...map[string]blackstart.InputValue) map[string]blackstart.InputValue {
	out := map[string]blackstart.InputValue{}
	for _, inputs := range maps {
		for key, value := range inputs {
			out[key] = value
		}
	}
	return out
}

// readIdentity reads certificate subject and SAN inputs from a module context.
func readIdentity(ctx blackstart.ModuleContext) (certIdentity, error) {
	subject, err := readSubject(ctx)
	if err != nil {
		return certIdentity{}, err
	}
	sans, err := readSANs(ctx)
	if err != nil {
		return certIdentity{}, err
	}
	return certIdentity{Subject: subject, SANs: sans}, nil
}

// readSubject reads optional X.509 subject inputs from a module context.
func readSubject(ctx blackstart.ModuleContext) (certSubject, error) {
	commonName, err := blackstart.ContextInputAs[string](ctx, inputCommonName, false)
	if err != nil {
		return certSubject{}, err
	}
	org, err := contextOptionalStringList(ctx, inputOrganization)
	if err != nil {
		return certSubject{}, err
	}
	orgUnit, err := contextOptionalStringList(ctx, inputOrganizationalU)
	if err != nil {
		return certSubject{}, err
	}
	country, err := contextOptionalStringList(ctx, inputCountry)
	if err != nil {
		return certSubject{}, err
	}
	locality, err := contextOptionalStringList(ctx, inputLocality)
	if err != nil {
		return certSubject{}, err
	}
	province, err := contextOptionalStringList(ctx, inputProvince)
	if err != nil {
		return certSubject{}, err
	}

	return certSubject{
		CommonName:         strings.TrimSpace(commonName),
		Organization:       normalizeOptionalStrings(org),
		OrganizationalUnit: normalizeOptionalStrings(orgUnit),
		Country:            normalizeOptionalStrings(country),
		Locality:           normalizeOptionalStrings(locality),
		Province:           normalizeOptionalStrings(province),
	}, nil
}

// readSANs reads optional X.509 SAN inputs from a module context.
func readSANs(ctx blackstart.ModuleContext) (certSANs, error) {
	dnsNames, err := contextOptionalStringList(ctx, inputDNSNames)
	if err != nil {
		return certSANs{}, err
	}
	ipRaw, err := contextOptionalStringList(ctx, inputIPAddresses)
	if err != nil {
		return certSANs{}, err
	}
	emailAddresses, err := contextOptionalStringList(ctx, inputEmailAddresses)
	if err != nil {
		return certSANs{}, err
	}
	uriRaw, err := contextOptionalStringList(ctx, inputURIs)
	if err != nil {
		return certSANs{}, err
	}

	ips, err := parseIPAddresses(ipRaw)
	if err != nil {
		return certSANs{}, err
	}
	uris, err := parseURIs(uriRaw)
	if err != nil {
		return certSANs{}, err
	}

	return certSANs{
		DNSNames:       normalizeOptionalStrings(dnsNames),
		IPAddresses:    ips,
		EmailAddresses: normalizeOptionalStrings(emailAddresses),
		URIs:           uris,
	}, nil
}

// contextOptionalStringList reads an optional string-or-list input from context.
func contextOptionalStringList(ctx blackstart.ModuleContext, key string) ([]string, error) {
	values, err := blackstart.ContextInputAs[[]string](ctx, key, false)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// normalizeOptionalStrings trims optional string list values and removes empty entries.
func normalizeOptionalStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// parseIPAddresses parses optional IP address SAN values.
func parseIPAddresses(values []string) ([]net.IP, error) {
	out := make([]net.IP, 0, len(values))
	for _, value := range normalizeOptionalStrings(values) {
		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP address %q", value)
		}
		out = append(out, ip)
	}
	return out, nil
}

// parseURIs parses optional URI SAN values.
func parseURIs(values []string) ([]*url.URL, error) {
	out := make([]*url.URL, 0, len(values))
	for _, value := range normalizeOptionalStrings(values) {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" {
			return nil, fmt.Errorf("invalid URI %q", value)
		}
		out = append(out, parsed)
	}
	return out, nil
}

// pkixName returns a standard library subject name.
func pkixName(subject certSubject) pkix.Name {
	return pkix.Name{
		CommonName:         subject.CommonName,
		Organization:       subject.Organization,
		OrganizationalUnit: subject.OrganizationalUnit,
		Country:            subject.Country,
		Locality:           subject.Locality,
		Province:           subject.Province,
	}
}

// normalizeProfile returns a canonical certificate profile name.
func normalizeProfile(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case profileServer, profileClient, profileServerClient, profileCA:
		return normalized, nil
	default:
		return "", fmt.Errorf(
			"invalid profile %q: allowed values are %s, %s, %s, %s",
			value, profileServer, profileClient, profileServerClient, profileCA,
		)
	}
}

// profileUsages returns key usage settings for a TLS profile.
func profileUsages(profile string) (x509.KeyUsage, []x509.ExtKeyUsage, bool, error) {
	switch profile {
	case profileServer:
		return x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, false, nil
	case profileClient:
		return x509.KeyUsageDigitalSignature,
			[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, false, nil
	case profileServerClient:
		return x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, false, nil
	case profileCA:
		return x509.KeyUsageCertSign | x509.KeyUsageCRLSign, nil, true, nil
	default:
		return 0, nil, false, fmt.Errorf("unsupported certificate profile %q", profile)
	}
}

// readValidityHours reads and validates certificate validity duration input.
func readValidityHours(ctx blackstart.ModuleContext) (int64, error) {
	validityHours, err := blackstart.ContextInputAs[int64](ctx, inputValidityHours, false)
	if err != nil {
		return 0, err
	}
	if validityHours == 0 {
		validityHours = defaultValidityHours
	}
	if validityHours < 1 {
		return 0, fmt.Errorf("validity_hours must be greater than zero")
	}
	return validityHours, nil
}

// parseCertificatePEM parses a single X.509 certificate PEM block.
func parseCertificatePEM(value string) (*x509.Certificate, error) {
	certs, err := parseCertificateChainPEM(value)
	if err != nil {
		return nil, err
	}
	if len(certs) != 1 {
		return nil, fmt.Errorf("certificate PEM must contain exactly one certificate")
	}
	return certs[0], nil
}

// parseCertificateChainPEM parses one or more X.509 certificate PEM blocks.
func parseCertificateChainPEM(value string) ([]*x509.Certificate, error) {
	remaining := []byte(value)
	var certs []*x509.Certificate
	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			if strings.TrimSpace(string(remaining)) != "" {
				return nil, fmt.Errorf("failed parsing certificate PEM")
			}
			break
		}
		if block.Type != pemTypeCertificate {
			return nil, fmt.Errorf("unsupported certificate PEM type %q", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing certificate: %w", err)
		}
		certs = append(certs, cert)
		remaining = rest
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("certificate PEM must contain at least one certificate")
	}
	return certs, nil
}

// parseCSRPEM parses a single X.509 certificate request PEM block.
func parseCSRPEM(value string) (*x509.CertificateRequest, error) {
	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, fmt.Errorf("failed parsing certificate request PEM")
	}
	if len(strings.TrimSpace(string(rest))) != 0 {
		return nil, fmt.Errorf("certificate request PEM must contain exactly one PEM block")
	}
	if block.Type != pemTypeCertRequest {
		return nil, fmt.Errorf("unsupported certificate request PEM type %q", block.Type)
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed parsing certificate request: %w", err)
	}
	if err = csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("certificate request signature is invalid: %w", err)
	}
	return csr, nil
}

// encodeCSRPEM returns a PEM-encoded X.509 certificate request.
func encodeCSRPEM(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertRequest, Bytes: der}))
}

// encodeCertificatePEM returns a PEM-encoded X.509 certificate.
func encodeCertificatePEM(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: der}))
}

// certificateFingerprintSHA256 returns a hex SHA-256 fingerprint of DER certificate bytes.
func certificateFingerprintSHA256(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// serialNumber returns a positive random certificate serial number.
func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), certificateSerialNumberBits)
	serial, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("failed generating serial number: %w", err)
	}
	if serial.Sign() == 0 {
		return big.NewInt(1), nil
	}
	return serial, nil
}

// certificateOutputs builds common certificate output values from generated DER and chain PEM.
func certificateOutputs(cert *x509.Certificate, der []byte, chainPEM string) certOutputs {
	certPEM := encodeCertificatePEM(der)
	combinedPEM := certPEM + chainPEM
	return certOutputs{
		PEM:          certPEM,
		ChainPEM:     chainPEM,
		CombinedPEM:  combinedPEM,
		SerialNumber: cert.SerialNumber.String(),
		NotBefore:    cert.NotBefore.Format(time.RFC3339),
		NotAfter:     cert.NotAfter.Format(time.RFC3339),
		SHA256:       certificateFingerprintSHA256(der),
	}
}

// outputCertificate writes common certificate outputs to module context.
func outputCertificate(ctx blackstart.ModuleContext, outputs certOutputs, includeChain bool) error {
	if err := ctx.Output(outputPEM, outputs.PEM); err != nil {
		return err
	}
	if includeChain {
		if err := ctx.Output(outputChainPEM, outputs.ChainPEM); err != nil {
			return err
		}
	}
	if err := ctx.Output(outputCombinedPEM, outputs.CombinedPEM); err != nil {
		return err
	}
	if err := ctx.Output(outputSerialNumber, outputs.SerialNumber); err != nil {
		return err
	}
	if err := ctx.Output(outputNotBefore, outputs.NotBefore); err != nil {
		return err
	}
	if err := ctx.Output(outputNotAfter, outputs.NotAfter); err != nil {
		return err
	}
	return ctx.Output(outputSHA256, outputs.SHA256)
}
