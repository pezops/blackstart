---
title: crypto_x509_certificate_request
---

# crypto_x509_certificate_request

Generates a PEM-encoded X.509 certificate signing request (CSR).

## Requirements

- The private key must be unencrypted and PEM encoded.

- TLS identities should be provided through subject alternative name inputs such as `dns_names` or
  `ip_addresses`.

## Inputs

| Id                  | Description                                                                                                             | Type             | Required |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| common_name         | Certificate subject common name. For TLS certificates, SANs should carry DNS names or IP addresses.                     | string           | false    |
| country             | Certificate subject country value or values.                                                                            | string, []string | false    |
| dns_names           | DNS subject alternative name value or values.                                                                           | string, []string | false    |
| email_addresses     | Email address subject alternative name value or values.                                                                 | string, []string | false    |
| ip_addresses        | IP address subject alternative name value or values.                                                                    | string, []string | false    |
| locality            | Certificate subject locality value or values.                                                                           | string, []string | false    |
| organization        | Certificate subject organization value or values.                                                                       | string, []string | false    |
| organizational_unit | Certificate subject organizational unit value or values.                                                                | string, []string | false    |
| private_key_pem     | PEM-encoded private key. Accepted formats: PKCS#8, PKCS#1 RSA, or SEC1 ECDSA. Encrypted private keys are not supported. | string           | true     |
| province            | Certificate subject state or province value or values.                                                                  | string, []string | false    |
| uris                | URI subject alternative name value or values.                                                                           | string, []string | false    |

## Outputs

| Id  | Description                                    | Type   |
| --- | ---------------------------------------------- | ------ |
| pem | PEM-encoded X.509 certificate signing request. | string |

## Examples

### Generate server CSR

```yaml
id: server-csr
module: crypto_x509_certificate_request
inputs:
  private_key_pem:
    fromDependency:
      id: server-key
      output: pem
  common_name: app.example.com
  dns_names:
    - app.example.com
    - www.app.example.com
```
