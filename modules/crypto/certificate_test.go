package crypto

import (
	"context"
	stdx509 "crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestCertificateTemplateDefaultValidity verifies the default validity duration is 46 days.
func TestCertificateTemplateDefaultValidity(t *testing.T) {
	now := time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC)
	template, err := certificateTemplate(profileServer, certIdentity{}, now, defaultValidityHours)
	require.NoError(t, err)
	require.Equal(t, 46*24*time.Hour, template.NotAfter.Sub(template.NotBefore))
}

// TestCertificateTemplateRejectsInvalidProfile verifies template profile errors.
func TestCertificateTemplateRejectsInvalidProfile(t *testing.T) {
	_, err := certificateTemplate("bad", certIdentity{}, time.Now(), defaultValidityHours)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported certificate profile")
}

// TestProfileUsages verifies TLS profiles map to expected X.509 usages.
func TestProfileUsages(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		wantCA   bool
		wantExt  stdx509.ExtKeyUsage
		wantCert bool
	}{
		{name: "server", profile: profileServer, wantExt: stdx509.ExtKeyUsageServerAuth},
		{name: "client", profile: profileClient, wantExt: stdx509.ExtKeyUsageClientAuth},
		{name: "server_client", profile: profileServerClient, wantExt: stdx509.ExtKeyUsageServerAuth},
		{name: "ca", profile: profileCA, wantCA: true, wantCert: true},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				keyUsage, extUsage, isCA, err := profileUsages(tt.profile)
				require.NoError(t, err)
				require.Equal(t, tt.wantCA, isCA)
				if tt.wantCert {
					require.Empty(t, extUsage)
					require.NotZero(t, keyUsage&stdx509.KeyUsageCertSign)
					return
				}
				require.Contains(t, extUsage, tt.wantExt)
				require.NotZero(t, keyUsage&stdx509.KeyUsageDigitalSignature)
			},
		)
	}

	_, _, _, err := profileUsages("unsupported")
	require.Error(t, err)
}

// TestPublicKeyHelpersRejectUnsupportedTypes verifies unsupported key helper branches.
func TestPublicKeyHelpersRejectUnsupportedTypes(t *testing.T) {
	_, err := supportedPrivateKey("not-a-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported private key type")

	_, err = publicKeyFromPrivateKey("not-a-key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported private key type")

	require.False(t, publicKeysEqual("not-a-public-key", "also-not-a-public-key"))
	require.False(t, publicKeysEqual(&testRSAKey(t).PublicKey, "not-a-public-key"))
}

// TestOutputCertificatePropagatesOutputErrors verifies output errors are returned.
func TestOutputCertificatePropagatesOutputErrors(t *testing.T) {
	ctx := &failingOutputContext{}
	err := outputCertificate(ctx, certOutputs{PEM: "cert"}, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "output failed")
}

// failingOutputContext fails every output write.
type failingOutputContext struct {
	blackstart.ModuleContext
}

// Output returns an error for output propagation tests.
func (f *failingOutputContext) Output(_ string, _ any) error {
	return fmt.Errorf("output failed")
}

// TestReadValidityHours verifies default, override, and invalid runtime values.
func TestReadValidityHours(t *testing.T) {
	defaultHours, err := readValidityHours(
		blackstart.OpContext(context.Background(), testOperation(moduleIDSelfSignedCert, nil)),
	)
	require.NoError(t, err)
	require.Equal(t, int64(defaultValidityHours), defaultHours)

	overrideHours, err := readValidityHours(
		blackstart.OpContext(
			context.Background(),
			testOperation(
				moduleIDSelfSignedCert,
				map[string]blackstart.Input{inputValidityHours: blackstart.NewInputFromValue(24)},
			),
		),
	)
	require.NoError(t, err)
	require.Equal(t, int64(24), overrideHours)

	_, err = readValidityHours(
		blackstart.OpContext(
			context.Background(),
			testOperation(
				moduleIDSelfSignedCert,
				map[string]blackstart.Input{inputValidityHours: blackstart.NewInputFromValue(-1)},
			),
		),
	)
	require.Error(t, err)
}
