package crypto

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestCertificateRequestValidateRejectsMissingPrivateKey verifies required private key validation.
func TestCertificateRequestValidateRejectsMissingPrivateKey(t *testing.T) {
	err := (&certificateRequestModule{}).Validate(
		*testOperation(moduleIDCertificateRequest, map[string]blackstart.Input{}),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), inputPrivateKeyPEM)
}

// TestValidationHelpersCoverStaticErrorBranches verifies static validation branch behavior.
func TestValidationHelpersCoverStaticErrorBranches(t *testing.T) {
	err := validateStaticCertificateChain(
		*testOperation(
			moduleIDSignedCert, map[string]blackstart.Input{
				inputCAChainPEM: blackstart.NewInputFromValue("not-pem"),
			},
		),
		inputCAChainPEM,
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), inputCAChainPEM)

	err = validateStaticValidityHours(
		*testOperation(
			moduleIDSelfSignedCert, map[string]blackstart.Input{
				inputValidityHours: blackstart.NewInputFromValue(-1),
			},
		),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), inputValidityHours)

	err = validateStaticSANs(
		*testOperation(
			moduleIDCertificateRequest, map[string]blackstart.Input{
				inputIPAddresses: blackstart.NewInputFromValue("not-an-ip"),
			},
		),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), inputIPAddresses)

	err = validateStaticSANs(
		*testOperation(
			moduleIDCertificateRequest, map[string]blackstart.Input{
				inputURIs: blackstart.NewInputFromValue("not-a-uri"),
			},
		),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), inputURIs)
}
