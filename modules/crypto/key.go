package crypto

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	pemTypePrivateKey    = "PRIVATE KEY"
	pemTypeRSAPrivateKey = "RSA PRIVATE KEY"
	pemTypeECPrivateKey  = "EC PRIVATE KEY"
	pemTypePublicKey     = "PUBLIC KEY"
)

// normalizeAlgorithm returns a canonical private key algorithm name.
func normalizeAlgorithm(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case algorithmRSA:
		return algorithmRSA, nil
	case algorithmECDSA:
		return algorithmECDSA, nil
	case algorithmED25519:
		return algorithmED25519, nil
	default:
		return "", fmt.Errorf("invalid algorithm %q: allowed values are RSA, ECDSA, ED25519", value)
	}
}

// normalizeRSABits validates and returns a supported RSA key size.
func normalizeRSABits(bits int64) (int, error) {
	switch bits {
	case 2048, 3072, 4096:
		return int(bits), nil
	default:
		return 0, fmt.Errorf("invalid rsa_bits %d: allowed values are 2048, 3072, 4096", bits)
	}
}

// normalizeECDSACurve returns a canonical ECDSA curve name.
func normalizeECDSACurve(value string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	switch normalized {
	case "P256":
		return "P256", nil
	case "P384":
		return "P384", nil
	case "P521":
		return "P521", nil
	default:
		return "", fmt.Errorf("invalid ecdsa_curve %q: allowed values are P256, P384, P521", value)
	}
}

// encodePrivateKeyPEM returns a PKCS#8 PEM-encoded private key.
func encodePrivateKeyPEM(key any) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("failed marshaling private key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: pemTypePrivateKey, Bytes: der})), nil
}

// encodePublicKeyPEM returns a SubjectPublicKeyInfo PEM-encoded public key.
func encodePublicKeyPEM(key any) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("failed marshaling public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: pemTypePublicKey, Bytes: der})), nil
}

// encodePublicKeyOpenSSH returns an OpenSSH authorized-key public key.
func encodePublicKeyOpenSSH(key any) (string, error) {
	publicKey, err := ssh.NewPublicKey(key)
	if err != nil {
		return "", fmt.Errorf("failed marshaling OpenSSH public key: %w", err)
	}
	return string(ssh.MarshalAuthorizedKey(publicKey)), nil
}

// publicKeyFingerprints returns MD5 and SHA256 OpenSSH public key fingerprints.
func publicKeyFingerprints(key any) (string, string, error) {
	publicKey, err := ssh.NewPublicKey(key)
	if err != nil {
		return "", "", fmt.Errorf("failed fingerprinting OpenSSH public key: %w", err)
	}
	return fingerprintMD5(publicKey), ssh.FingerprintSHA256(publicKey), nil
}

// fingerprintMD5 returns the legacy colon-separated MD5 OpenSSH fingerprint.
func fingerprintMD5(publicKey ssh.PublicKey) string {
	sum := md5.Sum(publicKey.Marshal())
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

// parsePrivateKeyPEM parses supported RSA, ECDSA, and Ed25519 private key PEM formats.
func parsePrivateKeyPEM(value string) (any, error) {
	block, rest := pem.Decode([]byte(value))
	if block == nil {
		return nil, fmt.Errorf("failed parsing private key PEM")
	}
	if len(strings.TrimSpace(string(rest))) != 0 {
		return nil, fmt.Errorf("private_key_pem must contain exactly one PEM block")
	}
	if isEncryptedPEMBlock(block) {
		return nil, fmt.Errorf("encrypted private keys are not supported")
	}

	switch block.Type {
	case pemTypePrivateKey:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing PKCS#8 private key: %w", err)
		}
		return supportedPrivateKey(key)
	case pemTypeRSAPrivateKey:
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing PKCS#1 RSA private key: %w", err)
		}
		return key, nil
	case pemTypeECPrivateKey:
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing SEC1 EC private key: %w", err)
		}
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key PEM type %q", block.Type)
	}
}

// isEncryptedPEMBlock reports whether a PEM block advertises legacy encryption.
func isEncryptedPEMBlock(block *pem.Block) bool {
	if block == nil {
		return false
	}
	if strings.EqualFold(block.Headers["Proc-Type"], "4,ENCRYPTED") {
		return true
	}
	_, hasDEKInfo := block.Headers["DEK-Info"]
	return hasDEKInfo
}

// supportedPrivateKey rejects parsed private keys outside RSA, ECDSA, and Ed25519.
func supportedPrivateKey(key any) (any, error) {
	switch key := key.(type) {
	case *rsa.PrivateKey:
		return key, nil
	case *ecdsa.PrivateKey:
		return key, nil
	case ed25519.PrivateKey:
		return key, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", key)
	}
}

// publicKeyFromPrivateKey returns the public key for a supported private key.
func publicKeyFromPrivateKey(key any) (any, error) {
	switch key := key.(type) {
	case *rsa.PrivateKey:
		return &key.PublicKey, nil
	case *ecdsa.PrivateKey:
		return &key.PublicKey, nil
	case ed25519.PrivateKey:
		return key.Public(), nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", key)
	}
}
