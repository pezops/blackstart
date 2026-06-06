package crypto

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"reflect"
	"time"

	"github.com/pezops/blackstart"
)

// certificateTemplate builds a certificate template from normalized inputs.
func certificateTemplate(
	profile string, identity certIdentity, now time.Time, validityHours int64,
) (*x509.Certificate, error) {
	keyUsage, extKeyUsage, isCA, err := profileUsages(profile)
	if err != nil {
		return nil, err
	}
	serial, err := serialNumber()
	if err != nil {
		return nil, err
	}
	notBefore := now.UTC()
	notAfter := notBefore.Add(time.Duration(validityHours) * time.Hour)

	return &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkixName(identity.Subject),
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		MaxPathLen:            caMaxPathLen(isCA),
		DNSNames:              identity.SANs.DNSNames,
		IPAddresses:           identity.SANs.IPAddresses,
		EmailAddresses:        identity.SANs.EmailAddresses,
		URIs:                  identity.SANs.URIs,
	}, nil
}

// caMaxPathLen returns a conservative path length for generated CA certificates.
func caMaxPathLen(isCA bool) int {
	if isCA {
		return 0
	}
	return -1
}

// certificateOutputInfo returns certificate output metadata.
func certificateOutputInfo(includeChain bool) map[string]blackstart.OutputValue {
	combinedDescription := "PEM-encoded certificate. For self-signed certificates this is the same as `pem`."
	if includeChain {
		combinedDescription = "PEM-encoded certificate followed by issuer chain PEM."
	}
	outputs := map[string]blackstart.OutputValue{
		outputPEM: {
			Description: "PEM-encoded X.509 certificate.",
			Type:        reflect.TypeFor[string](),
		},
		outputCombinedPEM: {
			Description: combinedDescription,
			Type:        reflect.TypeFor[string](),
		},
		outputSerialNumber: {
			Description: "Certificate serial number in base-10 format.",
			Type:        reflect.TypeFor[string](),
		},
		outputNotBefore: {
			Description: "Certificate validity start time in RFC3339 format.",
			Type:        reflect.TypeFor[string](),
		},
		outputNotAfter: {
			Description: "Certificate validity end time in RFC3339 format.",
			Type:        reflect.TypeFor[string](),
		},
		outputSHA256: {
			Description: "Hex-encoded SHA-256 fingerprint of the certificate DER bytes.",
			Type:        reflect.TypeFor[string](),
		},
	}
	if includeChain {
		outputs[outputChainPEM] = blackstart.OutputValue{
			Description: "PEM-encoded issuer chain, starting with the signing CA certificate.",
			Type:        reflect.TypeFor[string](),
		}
	}
	return outputs
}

// validateCAPrivateKeyMatchesCertificate verifies the CA key matches the CA certificate public key.
func validateCAPrivateKeyMatchesCertificate(key any, cert *x509.Certificate) error {
	publicKey, err := publicKeyFromPrivateKey(key)
	if err != nil {
		return err
	}
	if publicKeysEqual(publicKey, cert.PublicKey) {
		return nil
	}
	return fmt.Errorf("ca_private_key_pem does not match ca_certificate_pem")
}

// publicKeysEqual compares supported public key values.
func publicKeysEqual(a, b any) bool {
	aDER, err := x509.MarshalPKIXPublicKey(a)
	if err != nil {
		return false
	}
	bDER, err := x509.MarshalPKIXPublicKey(b)
	if err != nil {
		return false
	}
	return bytes.Equal(aDER, bDER)
}
