---
title: crypto_public_key
---

# crypto_public_key

Derives the public key from a supported private key.

## Requirements

- Private key must be a single, PEM-encoded block and in a PKCS#8, PKCS#1 RSA, or SEC1 ECDSA format.

- Passphrase-protected private keys are not supported - the `private_key_pem` input must be
  unencrypted.

## Inputs

| Id              | Description                                                                                                                                              | Type   | Required |
| --------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| private_key_pem | Private key PEM. Accepted formats: PKCS#8 private key, PKCS#1 RSA private key, SEC1 ECDSA private key. Supported PKCS#8 algorithms: RSA, ECDSA, Ed25519. | string | true     |

## Outputs

| Id      | Description                                         | Type   |
| ------- | --------------------------------------------------- | ------ |
| md5     | OpenSSH MD5 public key fingerprint.                 | string |
| openssh | OpenSSH authorized-key public key.                  | string |
| pem     | PEM-encoded SubjectPublicKeyInfo (SPKI) public key. | string |
| sha256  | OpenSSH SHA256 public key fingerprint.              | string |

## Examples

### Derive public key from dependency

```yaml
id: derive-public-key
module: crypto_public_key
inputs:
  private_key_pem:
    fromDependency:
      id: generate-private-key
      output: pem
```

### Key rotation with old and new Kubernetes Secret values

```yaml
operations:
  - id: k8s_client
    module: kubernetes_client

  - id: signing_key_secret
    module: kubernetes_secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: default
      name: signing-key

  - id: generate_new_private_key
    module: crypto_private_key
    inputs:
      algorithm: RSA
      rsa_bits: 4096

  - id: read_old_private_key
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_private_key
      value: ""
      update_policy: preserve

  - id: derive_old_public_key
    module: crypto_public_key
    inputs:
      private_key_pem:
        fromDependency:
          id: read_old_private_key
          output: value

  - id: old_private_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: old_private_key
      value:
        fromDependency:
          id: read_old_private_key
          output: value
      update_policy: overwrite

  - id: old_public_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: old_public_key
      value:
        fromDependency:
          id: derive_old_public_key
          output: openssh
      update_policy: overwrite

  - id: derive_new_public_key
    module: crypto_public_key
    inputs:
      private_key_pem:
        fromDependency:
          id: generate_new_private_key
          output: pem

  - id: new_private_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_private_key
      value:
        fromDependency:
          id: generate_new_private_key
          output: pem
      update_policy: overwrite
    dependsOn:
      - old_private_key_secret_value
      - old_public_key_secret_value

  - id: new_public_key_secret_value
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: signing_key_secret
          output: secret
      key: current_public_key
      value:
        fromDependency:
          id: derive_new_public_key
          output: openssh
      update_policy: overwrite
    dependsOn:
      - old_private_key_secret_value
      - old_public_key_secret_value
```
