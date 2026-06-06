---
title: crypto_private_key
---

# crypto_private_key

Generates a cryptographic private key.

Generated private keys are ephemeral. The output must be persisted in a storage operation if it is
needed after the workflow completes.

## Inputs

| Id          | Description                                                                                                                                                          | Type   | Required |
| ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| algorithm   | Private key algorithm. Allowed values: `RSA`, `ECDSA`, `ED25519`.                                                                                                    | string | true     |
| ecdsa_curve | ECDSA curve. Allowed values: `P256`, `P384`, `P521`. Aliases `P-256`, `P-384`, and `P-521` are accepted. Only used when `algorithm` is `ECDSA`.<br>Default: **P256** | string | false    |
| rsa_bits    | RSA key size in bits. Allowed values: `2048`, `3072`, `4096`. Only used when `algorithm` is `RSA`.<br>Default: **4096**                                              | int    | false    |

## Outputs

| Id     | Description                                                       | Type   |
| ------ | ----------------------------------------------------------------- | ------ |
| md5    | OpenSSH MD5 fingerprint derived from the generated public key.    | string |
| pem    | PEM-encoded private key in PKCS#8 format.                         | string |
| sha256 | OpenSSH SHA256 fingerprint derived from the generated public key. | string |

## Examples

### Generate ECDSA private key

```yaml
id: generate-ecdsa-key
module: crypto_private_key
inputs:
  algorithm: ECDSA
  ecdsa_curve: P256
```

### Generate Ed25519 private key

```yaml
id: generate-ed25519-key
module: crypto_private_key
inputs:
  algorithm: ED25519
```

### Generate RSA private key

```yaml
id: generate-rsa-key
module: crypto_private_key
inputs:
  algorithm: RSA
  rsa_bits: 4096
```
