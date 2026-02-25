package internal

import (
	"fmt"

	acmev1 "github.com/cert-manager/cert-manager/pkg/apis/acme/v1"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
	feature "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab/features"
)

const (
	specCertIssuerEmail  = "admin@example.com"
	specCertIssuerServer = "https://acme-v02.api.letsencrypt.org/directory"
)

// GetIngressIssuerConfig gets the ACME issuer to use from GitLab resource.
func GetIngressIssuerConfig(adapter gitlab.Adapter) certmanagerv1.IssuerConfig {
	solver := ingressAcmeSolver(adapter)

	if solver == nil {
		return certmanagerv1.IssuerConfig{
			SelfSigned: &certmanagerv1.SelfSignedIssuer{},
		}
	}

	email := adapter.Values().GetString("certmanager-issuer.email")
	if email == "" {
		email = specCertIssuerEmail
	}

	server := adapter.Values().GetString("certmanager-issuer.server")
	if server == "" {
		server = specCertIssuerServer
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
			Solvers: []acmev1.ACMEChallengeSolver{*solver},
		},
	}
}

// GetIngressIssuerConfig gets the ACME issuer to use from GitLab resource.
func GetGatewayIssuerConfig(adapter gitlab.Adapter) certmanagerv1.IssuerConfig {
	solver := gatewayAcmeSolver(adapter)

	if solver == nil {
		return certmanagerv1.IssuerConfig{
			SelfSigned: &certmanagerv1.SelfSignedIssuer{},
		}
	}

	email := adapter.Values().GetString("certmanager-issuer.email")
	if email == "" {
		email = specCertIssuerEmail
	}

	server := adapter.Values().GetString("certmanager-issuer.server")
	if server == "" {
		server = specCertIssuerServer
	}

	return certmanagerv1.IssuerConfig{
		ACME: &acmev1.ACMEIssuer{
			Email:  email,
			Server: server,
			PrivateKey: certmetav1.SecretKeySelector{
				LocalObjectReference: certmetav1.LocalObjectReference{
					Name: fmt.Sprintf("%s-gw-acme-key", adapter.ReleaseName()),
				},
			},
			Solvers: []acmev1.ACMEChallengeSolver{*solver},
		},
	}
}

// CertificateIngressIssuer creates a certificate generator for Ingress resources.
func CertificateIngressIssuer(adapter gitlab.Adapter) *certmanagerv1.Issuer {
	return &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CertificateIngressIssuerName(adapter),
			Namespace: adapter.Name().Namespace,
			Labels:    ingressIssuerLabels(adapter),
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: GetIngressIssuerConfig(adapter),
		},
	}
}

// CertificateGatewayIssuer creates a certificate generator for Gateway resources.
func CertificateGatewayIssuer(adapter gitlab.Adapter) *certmanagerv1.Issuer {
	return &certmanagerv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CertificateGatewayIssuerName(adapter),
			Namespace: adapter.Name().Namespace,
			Labels:    gatewayIssuerLabels(adapter),
		},
		Spec: certmanagerv1.IssuerSpec{
			IssuerConfig: GetGatewayIssuerConfig(adapter),
		},
	}
}

func CertificateIngressIssuerName(adapter gitlab.Adapter) string {
	return ingressIssuerLabels(adapter)["app.kubernetes.io/instance"]
}

func CertificateGatewayIssuerName(adapter gitlab.Adapter) string {
	return gatewayIssuerLabels(adapter)["app.kubernetes.io/instance"]
}

func ingressIssuerLabels(adapter gitlab.Adapter) map[string]string {
	return ResourceLabels(adapter.ReleaseName(), "issuer", GitlabType)
}

func gatewayIssuerLabels(adapter gitlab.Adapter) map[string]string {
	return ResourceLabels(adapter.ReleaseName(), "issuer-gw", GitlabType)
}

func ingressAcmeSolver(adapter gitlab.Adapter) *acmev1.ACMEChallengeSolver {
	if !adapter.WantsFeature(feature.ConfigureCertManager) {
		return nil
	}

	ingressClass := adapter.Values().GetString("global.ingress.class")
	if ingressClass == "" {
		ingressClass = fmt.Sprintf("%s-nginx", adapter.ReleaseName())
	}

	return &acmev1.ACMEChallengeSolver{
		Selector: &acmev1.CertificateDNSNameSelector{},
		HTTP01: &acmev1.ACMEChallengeSolverHTTP01{
			Ingress: &acmev1.ACMEChallengeSolverHTTP01Ingress{
				Class: &ingressClass,
			},
		},
	}
}

func gatewayAcmeSolver(adapter gitlab.Adapter) *acmev1.ACMEChallengeSolver {
	if !adapter.WantsFeature(feature.ConfigureGatewayCertManager) {
		return nil
	}

	namespace := gatewayv1.Namespace(
		adapter.Values().GetString("global.gatewayApi.gateway.namespace", adapter.Name().Namespace),
	)

	name := gatewayv1.ObjectName(
		adapter.Values().GetString("global.gatewayApi.gateway.name", fmt.Sprintf("%s-gw", adapter.Name())),
	)

	return &acmev1.ACMEChallengeSolver{
		Selector: &acmev1.CertificateDNSNameSelector{},
		HTTP01: &acmev1.ACMEChallengeSolverHTTP01{
			GatewayHTTPRoute: &acmev1.ACMEChallengeSolverHTTP01GatewayHTTPRoute{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      name,
						Namespace: &namespace,
					},
				},
			},
		},
	}
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
	usesExternalIngressCert := adapter.Values().GetString("global.ingress.tls.secretName") == ""
	enabledGatewayCertmanagerCerts := adapter.WantsFeature(feature.ConfigureGatewayCertManager)

	return EndpointTLS{
		gitlab:   usesExternalIngressCert || enabledGatewayCertmanagerCerts,
		registry: usesExternalIngressCert || enabledGatewayCertmanagerCerts,
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
