---
title: crypto_x509_signed_certificate
---

# crypto_x509_signed_certificate

Signs a PEM-encoded X.509 certificate signing request using a local CA certificate and private key.

## Requirements

- The CSR must be PEM encoded and have a valid signature.

- The CA certificate must be a PEM-encoded CA certificate.

- The CA private key must be unencrypted and PEM encoded.

- This module does not assert public-trust CA compliance.

## Inputs

| Id                 | Description                                                                                                                                                                                                                                                                                                                 | Type   | Required |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| ca_certificate_pem | PEM-encoded CA certificate used as the issuer.                                                                                                                                                                                                                                                                              | string | true     |
| ca_chain_pem       | Optional PEM-encoded certificates to append after the signing CA in chain outputs.                                                                                                                                                                                                                                          | string | false    |
| ca_private_key_pem | PEM-encoded CA private key used to sign the certificate. Encrypted private keys are not supported.                                                                                                                                                                                                                          | string | true     |
| csr_pem            | PEM-encoded X.509 certificate signing request.                                                                                                                                                                                                                                                                              | string | true     |
| profile            | TLS certificate profile for the issued certificate. Allowed values: `server`, `client`, `server_client`, `ca`.<br>Default: **server**                                                                                                                                                                                       | string | false    |
| validity_hours     | Certificate validity period in hours. Defaults to 46 days (1104 hours), aligned with the future CA/Browser Forum 46-day recommendation in section 6.3.2: https://cabforum.org/working-groups/server/baseline-requirements/requirements/#632-certificate-operational-periods-and-key-pair-usage-periods<br>Default: **1104** | int    | false    |

## Outputs

| Id            | Description                                                         | Type   |
| ------------- | ------------------------------------------------------------------- | ------ |
| chain_pem     | PEM-encoded issuer chain, starting with the signing CA certificate. | string |
| combined_pem  | PEM-encoded certificate followed by issuer chain PEM.               | string |
| not_after     | Certificate validity end time in RFC3339 format.                    | string |
| not_before    | Certificate validity start time in RFC3339 format.                  | string |
| pem           | PEM-encoded X.509 certificate.                                      | string |
| serial_number | Certificate serial number in base-10 format.                        | string |
| sha256        | Hex-encoded SHA-256 fingerprint of the certificate DER bytes.       | string |

## Examples

### Sign server CSR with local CA

```yaml
operations:
  - id: ca_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: ca_cert
    module: crypto_x509_self_signed_certificate
    inputs:
      private_key_pem:
        fromDependency:
          id: ca_key
          output: pem
      profile: ca
      common_name: Example Internal CA

  - id: server_key
    module: crypto_private_key
    inputs:
      algorithm: ED25519

  - id: server_csr
    module: crypto_x509_certificate_request
    inputs:
      private_key_pem:
        fromDependency:
          id: server_key
          output: pem
      common_name: app.example.com
      dns_names:
        - app.example.com

  - id: server_cert
    module: crypto_x509_signed_certificate
    inputs:
      csr_pem:
        fromDependency:
          id: server_csr
          output: pem
      ca_certificate_pem:
        fromDependency:
          id: ca_cert
          output: pem
      ca_private_key_pem:
        fromDependency:
          id: ca_key
          output: pem
      profile: server
```
