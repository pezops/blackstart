package crypto

import "github.com/pezops/blackstart"

const (
	inputAlgorithm     = "algorithm"
	inputRSABits       = "rsa_bits"
	inputECDSACurve    = "ecdsa_curve"
	inputPrivateKeyPEM = "private_key_pem"

	outputPEM     = "pem"
	outputOpenSSH = "openssh"
	outputMD5     = "md5"
	outputSHA256  = "sha256"
)

const (
	algorithmRSA     = "RSA"
	algorithmECDSA   = "ECDSA"
	algorithmED25519 = "ED25519"
)

func init() {
	blackstart.RegisterPathName("crypto", "Cryptography")
}
