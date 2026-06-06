package crypto

import "github.com/pezops/blackstart"

const (
	moduleIDCertificateRequest  = "crypto_x509_certificate_request"
	moduleIDSelfSignedCert      = "crypto_x509_self_signed_certificate"
	moduleIDSignedCert          = "crypto_x509_signed_certificate"
	defaultValidityHours        = 1104
	cabForumValiditySectionURL  = "https://cabforum.org/working-groups/server/baseline-requirements/requirements/#632-certificate-operational-periods-and-key-pair-usage-periods"
	certificateSerialNumberBits = 128

	profileServer       = "server"
	profileClient       = "client"
	profileServerClient = "server_client"
	profileCA           = "ca"
	pemTypeCertificate  = "CERTIFICATE"
	pemTypeCertRequest  = "CERTIFICATE REQUEST"
)

func init() {
	blackstart.RegisterModule(moduleIDCertificateRequest, NewCertificateRequest)
	blackstart.RegisterModule(moduleIDSelfSignedCert, NewSelfSignedCertificate)
	blackstart.RegisterModule(moduleIDSignedCert, NewSignedCertificate)
}
