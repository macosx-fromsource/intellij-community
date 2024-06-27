package internal

import (
	"fmt"

	acmev1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
)

const (
	specCertIssuerEmail  = "admin@example.com"
	specCertIssuerServer = "https://acme-v02.api.letsencrypt.org/directory"
)

// GetIssuerConfig gets the ACME issuer to use from GitLab resource.
func GetIssuerConfig(adapter gitlab.Adapter) certmanagerv1.IssuerConfig {
	if adapter.WantsFeature(feature.ConfigureCertManager) {
		email := adapter.Values().GetString("certmanager-issuer.email")
		if email == "" {
			email = specCertIssuerEmail
		}

		server := adapter.Values().GetString("certmanager-issuer.server")
		if server == "" {
			server = specCertIssuerServer
		}

		ingressClass := adapter.Values().GetString("global.ingress.class")
		if ingressClass == "" {
			ingressClass = fmt.Sprintf("%s-nginx", adapter.ReleaseName())
		}

		return certmanagerv1.IssuerConfig{
			ACME: &acmev1.ACMEIssuer{
				Email:  email,
				Server: server,
				PrivateKey: certmetav1.SecretKeySelector{
					LocalObjectReference: certmetav1.LocalObjectReference{
						Name: fmt.Sprintf("%s-acme-key", adapter.ReleaseName()),
					},
				},
				Solvers: []acmev1.ACMEChallengeSolver{
					{
						Selector: &acmev1.CertificateDNSNameSelector{},
						HTTP01: &acmev1.ACMEChallengeSolverHTTP01{
							Ingress: &acmev1.ACMEChallengeSolverHTTP01Ingress{
								Class: &ingressClass,
							},
						},
					},
				},
			},
		}
	}

	return certmanagerv1.IssuerConfig{
		SelfSigned: &certmanagerv1.SelfSignedIssuer{},
	}
}

// CertificateIssuer create a certificate generator.
func CertificateIssuer(adapter gitlab.Adapter) *certmanagerv1.Issuer {
	labels := ResourceLabels(adapter.ReleaseName(), "issuer", GitlabType)

	issuer := &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      labels["app.kubernetes.io/instance"],
			Namespace: adapter.Name().Namespace,
			Labels:    labels,
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: GetIssuerConfig(adapter),
		},
	}

	return issuer
}

// EndpointTLS informs which services require
// to be secured using generated TLS certificates.
type EndpointTLS struct {
	gitlab   bool
	registry bool
	minio    bool
}

// RequiresCertManagerCertificate function returns true when an administrator
// did not provide a TLS ceritificate for an endpoint.
func RequiresCertManagerCertificate(adapter gitlab.Adapter) EndpointTLS {
	// This implies that Operator can only consume wildcard certificate and individual certificate
	// per service will be ignored.
	tlsSecretName := adapter.Values().GetString("global.ingress.tls.secretName")

	return EndpointTLS{
		gitlab:   tlsSecretName == "",
		registry: tlsSecretName == "",
	}
}

// GitLab returns true if GitLab endpoint requires
// a cert-manager provisioned certificate.
func (ep EndpointTLS) GitLab() bool {
	return ep.gitlab
}

// Registry returns true if Registry endpoint requires
// a cert-manager provisioned certificate.
func (ep EndpointTLS) Registry() bool {
	return ep.registry
}

// Minio returns true if Minio endpoint requires
// a cert-manager provisioned certificate.
func (ep EndpointTLS) Minio() bool {
	return ep.minio
}

// Any returns true if any ingress requires
// a cert-manager certificate.
func (ep EndpointTLS) Any() bool {
	return ep.gitlab || ep.registry || ep.minio
}
