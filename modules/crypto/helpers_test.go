package crypto

import (
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestNormalizeProfile verifies certificate profile normalization.
func TestNormalizeProfile(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "server", value: "server", want: profileServer},
		{name: "client_upper", value: " CLIENT ", want: profileClient},
		{name: "server_client", value: "server_client", want: profileServerClient},
		{name: "ca", value: "ca", want: profileCA},
		{name: "invalid", value: "code_signing", wantErr: "allowed values"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				got, err := normalizeProfile(tt.value)
				if tt.wantErr == "" {
					require.NoError(t, err)
					require.Equal(t, tt.want, got)
					return
				}
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			},
		)
	}
}

// TestParseSubjectAlternativeNames verifies SAN parsers accept supported values and reject invalid values.
func TestParseSubjectAlternativeNames(t *testing.T) {
	ips, err := parseIPAddresses([]string{"127.0.0.1", " ::1 "})
	require.NoError(t, err)
	require.Len(t, ips, 2)
	require.Equal(t, net.ParseIP("127.0.0.1"), ips[0])

	_, err = parseIPAddresses([]string{"not-an-ip"})
	require.Error(t, err)

	uris, err := parseURIs([]string{"spiffe://example.test/service"})
	require.NoError(t, err)
	require.Equal(t, "spiffe", uris[0].Scheme)

	_, err = parseURIs([]string{"no-scheme"})
	require.Error(t, err)
}

// TestReadIdentityReadsAllFields verifies subject and SAN values are normalized from context.
func TestReadIdentityReadsAllFields(t *testing.T) {
	op := testOperation(
		moduleIDCertificateRequest,
		map[string]blackstart.Input{
			inputCommonName:      blackstart.NewInputFromValue(" app.example.com "),
			inputOrganization:    blackstart.NewInputFromValue([]string{" Example ", ""}),
			inputOrganizationalU: blackstart.NewInputFromValue(" Platform "),
			inputCountry:         blackstart.NewInputFromValue("US"),
			inputLocality:        blackstart.NewInputFromValue("Boston"),
			inputProvince:        blackstart.NewInputFromValue("MA"),
			inputDNSNames:        blackstart.NewInputFromValue([]string{"app.example.com", ""}),
			inputIPAddresses:     blackstart.NewInputFromValue("127.0.0.1"),
			inputEmailAddresses:  blackstart.NewInputFromValue("admin@example.com"),
			inputURIs:            blackstart.NewInputFromValue("spiffe://example.test/app"),
		},
	)

	identity, err := readIdentity(blackstart.OpContext(context.Background(), op))
	require.NoError(t, err)
	require.Equal(t, "app.example.com", identity.Subject.CommonName)
	require.Equal(t, []string{"Example"}, identity.Subject.Organization)
	require.Equal(t, []string{"Platform"}, identity.Subject.OrganizationalUnit)
	require.Equal(t, []string{"US"}, identity.Subject.Country)
	require.Equal(t, []string{"Boston"}, identity.Subject.Locality)
	require.Equal(t, []string{"MA"}, identity.Subject.Province)
	require.Equal(t, []string{"app.example.com"}, identity.SANs.DNSNames)
	require.Equal(t, net.ParseIP("127.0.0.1"), identity.SANs.IPAddresses[0])
	require.Equal(t, []string{"admin@example.com"}, identity.SANs.EmailAddresses)
	require.Equal(t, "spiffe", identity.SANs.URIs[0].Scheme)
}

// TestPrivateKeyPEMParsingBranches verifies accepted formats and parser errors.
func TestPrivateKeyPEMParsingBranches(t *testing.T) {
	rsaKey := testRSAKey(t)
	ecdsaKey := testECDSAKey(t)
	edKey := testED25519Key(t)
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeRSAPrivateKey, Bytes: stdx509.MarshalPKCS1PrivateKey(rsaKey)}))
	sec1DER, err := stdx509.MarshalECPrivateKey(ecdsaKey)
	require.NoError(t, err)
	sec1PEM := string(pem.EncodeToMemory(&pem.Block{Type: pemTypeECPrivateKey, Bytes: sec1DER}))

	for _, value := range []string{
		encodeTestPrivateKeyPEM(t, rsaKey),
		pkcs1PEM,
		sec1PEM,
		encodeTestPrivateKeyPEM(t, edKey),
	} {
		_, err := parsePrivateKeyPEM(value)
		require.NoError(t, err)
	}

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{name: "malformed", value: "not-pem", wantErr: "failed parsing private key PEM"},
		{name: "multiple_blocks", value: encodeTestPrivateKeyPEM(t, rsaKey) + encodeTestPrivateKeyPEM(
			t, rsaKey,
		), wantErr: "must contain exactly one PEM block"},
		{
			name:    "encrypted_proc_type",
			value:   string(pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Headers: map[string]string{"Proc-Type": "4,ENCRYPTED"}})),
			wantErr: "encrypted private keys are not supported",
		},
		{
			name:    "encrypted_dek_info",
			value:   string(pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Headers: map[string]string{"DEK-Info": "AES-256-CBC,00"}})),
			wantErr: "encrypted private keys are not supported",
		},
		{
			name:    "unsupported_type",
			value:   string(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: []byte("invalid")})),
			wantErr: "unsupported private key PEM type",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, err := parsePrivateKeyPEM(tt.value)
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
			},
		)
	}
}

// TestCertificateAndCSRParsingRejectsInvalidInputs verifies parser error branches.
func TestCertificateAndCSRParsingRejectsInvalidInputs(t *testing.T) {
	_, err := parseCertificatePEM("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate PEM must contain at least one certificate")

	_, err = parseCertificatePEM(
		string(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertRequest, Bytes: []byte("invalid")})),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported certificate PEM type")

	certPEM := runModule(
		t,
		&selfSignedCertificateModule{},
		moduleIDSelfSignedCert,
		map[string]blackstart.Input{
			inputPrivateKeyPEM: blackstart.NewInputFromValue(encodeTestPrivateKeyPEM(t, testRSAKey(t))),
		},
	)[outputPEM]
	_, err = parseCertificatePEM(certPEM + certPEM)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exactly one certificate")

	_, err = parseCSRPEM("not-pem")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed parsing certificate request PEM")

	_, err = parseCSRPEM(
		string(pem.EncodeToMemory(&pem.Block{Type: pemTypeCertificate, Bytes: []byte("invalid")})),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported certificate request PEM type")
}
