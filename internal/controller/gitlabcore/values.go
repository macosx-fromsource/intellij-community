package gitlabcore

import (
	"strings"

	"github.com/mitchellh/copystructure"
	"github.com/pkg/errors"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// The chart value keys the structured specification maps onto.
const (
	installCertmanagerKey = "installCertmanager"
	installRunnerKey      = "gitlab-runner.install"

	// The subchart conditions of the bundled ingress controllers, as
	// Chart.yaml declares them.
	installNGINXKey    = "nginx-ingress.enabled"
	installNGINXGeoKey = "nginx-ingress-geo.enabled"
	installHAProxyKey  = "haproxy.install"
	installTraefikKey  = "traefik.install"
	installEnvoyKey    = "global.gatewayApi.installEnvoy"
	hostsDomainKey     = "global.hosts.domain"
	hostsGitLabNameKey = "global.hosts.gitlab.name"
	// These two name where the license lives, not the license itself.
	licenseSecretKey = "global.gitlab.license.secret" //nolint:gosec // A chart value path, not a credential.
	licenseKeyKey    = "global.gitlab.license.key"

	sharedSecretsCreateRBACKey = "shared-secrets.rbac.create"
	sharedSecretsCreateSAKey   = "shared-secrets.serviceAccount.create"
	sharedSecretsSANameKey     = "shared-secrets.serviceAccount.name"
	sharedSecretsRunAsUserKey  = "shared-secrets.securityContext.runAsUser"
	sharedSecretsFSGroupKey    = "shared-secrets.securityContext.fsGroup"
)

// EffectiveValues returns the values the chart is rendered with, in three
// layers:
//
//  1. The values derived from the structured fields of the specification.
//  2. spec.chart.values, merged over them. The free-form values win here, as
//     ADR 26 decides: a structured field that maps to the same key as a value an
//     administrator sets does not override that choice, which keeps the
//     free-form values a working escape hatch.
//  3. The Operator overrides, which win over everything. They are the settings
//     an instance may not choose, because the Operator, not the chart, owns
//     what they configure.
//
// The free-form values are copied before they are merged. They come from the
// informer cache, and both the merge and the renderer write into the map they
// are given.
func EffectiveValues(core *apiv2alpha1.GitLabCore) (support.Values, error) {
	values := support.Values{}

	if err := setSharedSecretsValues(values); err != nil {
		return nil, err
	}

	if err := setHostnameValues(values, core.Spec.Hostname); err != nil {
		return nil, err
	}

	if err := setLicenseValues(values, core.Spec.License); err != nil {
		return nil, err
	}

	if err := mergeUserValues(values, core); err != nil {
		return nil, err
	}

	return values, setOverrideValues(values)
}

// mergeUserValues merges spec.chart.values over the derived values.
func mergeUserValues(values support.Values, core *apiv2alpha1.GitLabCore) error {
	if core.Spec.Chart.Values.Object == nil {
		return nil
	}

	userValues, err := copystructure.Copy(core.Spec.Chart.Values.Object)
	if err != nil {
		return errors.Wrap(err, "failed to copy spec.chart.values")
	}

	typedValues, ok := userValues.(map[string]interface{})
	if !ok {
		return errors.New("spec.chart.values is not an object")
	}

	if err := values.Merge(typedValues); err != nil {
		return errors.Wrap(err, "failed to merge spec.chart.values")
	}

	return nil
}

// overrideValues are the settings an instance may not choose, because the
// Operator, not the chart, owns what they configure.
//
//   - cert-manager is a prerequisite of the Operator: whoever administers the
//     cluster installs it once, and it serves every instance. A release that
//     installed its own would bring a second controller for the same
//     cluster-wide APIs, plus the definitions and RBAC the Operator does not
//     apply, so the copy would not work anyway.
//   - The GitLab Runner has a lifecycle of its own, is released on its own
//     schedule, and is deployed from its own chart. The umbrella chart is not
//     where an instance gets one.
//   - No ingress controller is installed. The bundled NGINX, HAProxy, and
//     Traefik subcharts are deprecated, and Envoy Gateway is a cluster-wide
//     controller like cert-manager. The chart still routes to whatever the
//     cluster serves: global.ingress and global.gatewayApi.enabled stay open,
//     only the subcharts that would install a controller are shut.
//
// Keep this list short. Every entry is a value an administrator sets in
// spec.chart.values and does not get.
var overrideValues = map[string]interface{}{
	installCertmanagerKey: false,
	installRunnerKey:      false,
	installNGINXKey:       false,
	installNGINXGeoKey:    false,
	installHAProxyKey:     false,
	installTraefikKey:     false,
	installEnvoyKey:       false,
}

// setOverrideValues applies the Operator overrides over everything else.
func setOverrideValues(values support.Values) error {
	for key, value := range overrideValues {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
	}

	return nil
}

// setSharedSecretsValues mirrors the shared secrets defaults of the v1beta1
// controller.
//
// The Job runs under the ServiceAccount of the Operator, the one
// GITLAB_MANAGER_SERVICE_ACCOUNT names, rather than under an account and a Role
// the chart creates. The Operator installation provisions that account with the
// permissions the Job needs, and the Operator creates neither accounts nor RBAC
// of its own. The account therefore has to exist in the namespace of the
// resource, with permission to manage Secrets there, before the hooks run.
//
// Turning the chart RBAC off keeps it out of the render, rather than leaving
// runHooks to filter it on the way to the cluster. The v1beta1 controller
// reaches the same result by applying only the ConfigMap and the Job of the
// component.
//
// The empty security context values keep the Job compatible with the OpenShift
// nonroot SecurityContextConstraint, which assigns the user and the group itself
// and rejects a Pod that pins them.
//
// These are defaults rather than overrides, so spec.chart.values can still
// replace them, for example with a service account of its own.
func setSharedSecretsValues(values support.Values) error {
	defaults := map[string]interface{}{
		sharedSecretsCreateRBACKey: false,
		sharedSecretsCreateSAKey:   false,
		sharedSecretsSANameKey:     settings.ManagerServiceAccount,
		sharedSecretsRunAsUserKey:  "",
		sharedSecretsFSGroupKey:    "",
	}

	for key, value := range defaults {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
	}

	return nil
}

// setHostnameValues maps spec.hostname onto the chart host values.
//
// The hostname alone does not configure the chart: it names the GitLab host,
// while every other component host hangs off global.hosts.domain. A hostname
// that carries a subdomain therefore also sets the domain, so that
// gitlab.example.com yields registry.example.com and kas.example.com beside
// it, instead of the chart default of example.com.
//
// An apex hostname is its own domain, because stripping its first label would
// leave the public suffix. Its component hosts then read as registry.example.com
// beside a GitLab host of example.com.
func setHostnameValues(values support.Values, hostname string) error {
	if hostname == "" {
		return nil
	}

	if err := values.SetValue(hostsGitLabNameKey, hostname); err != nil {
		return errors.Wrapf(err, "failed to set %s", hostsGitLabNameKey)
	}

	domain := hostname

	if labels := strings.SplitN(hostname, ".", 2); len(labels) == 2 && strings.Contains(labels[1], ".") {
		domain = labels[1]
	}

	if err := values.SetValue(hostsDomainKey, domain); err != nil {
		return errors.Wrapf(err, "failed to set %s", hostsDomainKey)
	}

	return nil
}

// setLicenseValues points the chart at the Secret that holds the license. The
// key is never read here: the license reaches GitLab through the Secret, so no
// license key passes through the Operator.
func setLicenseValues(values support.Values, license *apiv2alpha1.LicenseSpec) error {
	if license == nil {
		return nil
	}

	if err := values.SetValue(licenseSecretKey, license.SecretRef.Name); err != nil {
		return errors.Wrapf(err, "failed to set %s", licenseSecretKey)
	}

	if err := values.SetValue(licenseKeyKey, license.SecretRef.Key); err != nil {
		return errors.Wrapf(err, "failed to set %s", licenseKeyKey)
	}

	return nil
}
