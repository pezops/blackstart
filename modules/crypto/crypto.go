package crypto

import (
	"github.com/pezops/blackstart"
	_ "github.com/pezops/blackstart/util"
)

const (
	inputAlgorithm        = "algorithm"
	inputRSABits          = "rsa_bits"
	inputECDSACurve       = "ecdsa_curve"
	inputPrivateKeyPEM    = "private_key_pem"
	inputCSRPEM           = "csr_pem"
	inputCACertificatePEM = "ca_certificate_pem"
	inputCAPrivateKeyPEM  = "ca_private_key_pem"
	inputCAChainPEM       = "ca_chain_pem"
	inputProfile          = "profile"
	inputCommonName       = "common_name"
	inputOrganization     = "organization"
	inputOrganizationalU  = "organizational_unit"
	inputCountry          = "country"
	inputLocality         = "locality"
	inputProvince         = "province"
	inputDNSNames         = "dns_names"
	inputIPAddresses      = "ip_addresses"
	inputEmailAddresses   = "email_addresses"
	inputURIs             = "uris"
	inputValidityHours    = "validity_hours"

	outputPEM          = "pem"
	outputOpenSSH      = "openssh"
	outputMD5          = "md5"
	outputSHA256       = "sha256"
	outputChainPEM     = "chain_pem"
	outputCombinedPEM  = "combined_pem"
	outputSerialNumber = "serial_number"
	outputNotBefore    = "not_before"
	outputNotAfter     = "not_after"
)

const (
	algorithmRSA     = "RSA"
	algorithmECDSA   = "ECDSA"
	algorithmED25519 = "ED25519"
)

func init() {
	blackstart.RegisterPathName("crypto", "Cryptography")
}
