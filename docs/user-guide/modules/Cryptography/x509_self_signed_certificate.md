---
title: crypto_x509_self_signed_certificate
---

# crypto_x509_self_signed_certificate

Generates a PEM-encoded self-signed X.509 certificate.

## Requirements

- The private key must be unencrypted and PEM encoded.

- TLS identities should be provided through subject alternative name inputs such as `dns_names` or
  `ip_addresses`.

- This module does not assert public-trust CA compliance.

## Inputs

| Id                  | Description                                                                                                                                                                                                                                                                                                                 | Type             | Required |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| common_name         | Certificate subject common name. For TLS certificates, SANs should carry DNS names or IP addresses.                                                                                                                                                                                                                         | string           | false    |
| country             | Certificate subject country value or values.                                                                                                                                                                                                                                                                                | string, []string | false    |
| dns_names           | DNS subject alternative name value or values.                                                                                                                                                                                                                                                                               | string, []string | false    |
| email_addresses     | Email address subject alternative name value or values.                                                                                                                                                                                                                                                                     | string, []string | false    |
| ip_addresses        | IP address subject alternative name value or values.                                                                                                                                                                                                                                                                        | string, []string | false    |
| locality            | Certificate subject locality value or values.                                                                                                                                                                                                                                                                               | string, []string | false    |
| organization        | Certificate subject organization value or values.                                                                                                                                                                                                                                                                           | string, []string | false    |
| organizational_unit | Certificate subject organizational unit value or values.                                                                                                                                                                                                                                                                    | string, []string | false    |
| private_key_pem     | PEM-encoded private key. Accepted formats: PKCS#8, PKCS#1 RSA, or SEC1 ECDSA. Encrypted private keys are not supported.                                                                                                                                                                                                     | string           | true     |
| profile             | TLS certificate profile. Allowed values: `server`, `client`, `server_client`, `ca`.<br>Default: **server**                                                                                                                                                                                                                  | string           | false    |
| province            | Certificate subject state or province value or values.                                                                                                                                                                                                                                                                      | string, []string | false    |
| uris                | URI subject alternative name value or values.                                                                                                                                                                                                                                                                               | string, []string | false    |
| validity_hours      | Certificate validity period in hours. Defaults to 46 days (1104 hours), aligned with the future CA/Browser Forum 46-day recommendation in section 6.3.2: https://cabforum.org/working-groups/server/baseline-requirements/requirements/#632-certificate-operational-periods-and-key-pair-usage-periods<br>Default: **1104** | int              | false    |

## Outputs

| Id            | Description                                                                      | Type   |
| ------------- | -------------------------------------------------------------------------------- | ------ |
| combined_pem  | PEM-encoded certificate. For self-signed certificates this is the same as `pem`. | string |
| not_after     | Certificate validity end time in RFC3339 format.                                 | string |
| not_before    | Certificate validity start time in RFC3339 format.                               | string |
| pem           | PEM-encoded X.509 certificate.                                                   | string |
| serial_number | Certificate serial number in base-10 format.                                     | string |
| sha256        | Hex-encoded SHA-256 fingerprint of the certificate DER bytes.                    | string |

## Examples

### Generate self-signed server certificate

```yaml
operations:
  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      profile: server
      common_name: app.example.com
      dns_names:
        - app.example.com
```

### Store self-signed certificate in a Kubernetes TLS Secret

```yaml
operations:
  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      profile: server
      common_name: app.example.com
      dns_names:
        - app.example.com

  - id: k8s_client
    module: kubernetes_client

  - id: tls_secret
    module: kubernetes_secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: default
      name: app-tls
      type: kubernetes.io/tls

  - id: tls_crt
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: tls_secret
          output: secret
      key: tls.crt
      value:
        fromDependency:
          id: server_cert
          output: pem
      update_policy: overwrite

  - id: tls_key
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: tls_secret
          output: secret
      key: tls.key
      value:
        fromDependency:
          id: server_key
          output: pem
      update_policy: overwrite
```
