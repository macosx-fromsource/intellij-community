package gitlabcore

import (
	"strings"

	"github.com/pkg/errors"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/controller/release"
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
	editionKey         = "global.edition"
	// These two name where the license lives, not the license itself.
	licenseSecretKey = "global.gitlab.license.secret" //nolint:gosec // A chart value path, not a credential.
	licenseKeyKey    = "global.gitlab.license.key"

	// The connections to the external data stores. The password keys name where a password lives,
	// not the password itself.
	psqlHostKey           = "global.psql.host"
	psqlPasswordSecretKey = "global.psql.password.secret" //nolint:gosec // A chart value path, not a credential.
	psqlPasswordKeyKey    = "global.psql.password.key"    //nolint:gosec // A chart value path, not a credential.
	redisHostKey          = "global.redis.host"
	redisAuthSecretKey    = "global.redis.auth.secret" //nolint:gosec // A chart value path, not a credential.
	redisAuthKeyKey       = "global.redis.auth.key"

	// The subchart toggle of the container registry, as its own values declare it.
	registryEnabledKey = "registry.enabled"

	objectStoreEnabledKey          = "global.appConfig.object_store.enabled"
	objectStoreConnectionSecretKey = "global.appConfig.object_store.connection.secret" //nolint:gosec // A chart value path, not a credential.
	objectStoreConnectionKeyKey    = "global.appConfig.object_store.connection.key"

	// OpenBao: the GitLab-side integration, the bundled subchart, and the PostgreSQL database
	// it needs of its own (never the main application database; see setOpenBaoValues).
	openbaoEnabledKey            = "global.openbao.enabled"
	openbaoInstallKey            = "openbao.install"
	openbaoPsqlHostKey           = "global.openbao.psql.host"
	openbaoPsqlPortKey           = "global.openbao.psql.port"
	openbaoPsqlDatabaseKey       = "global.openbao.psql.database"
	openbaoPsqlUsernameKey       = "global.openbao.psql.username"
	openbaoPsqlPasswordSecretKey = "global.openbao.psql.password.secret"
	openbaoPsqlPasswordKeyKey    = "global.openbao.psql.password.key"

	// The ServiceAccount OpenBao's pod runs as, and the chart's own ServiceAccount/Role it is
	// defaulted off in favor of: the Operator does not manage RBAC on the cluster it
	// reconciles, so the Role granting get/update/patch on Pods, and the RoleBinding to it, are
	// an administrator prerequisite (see setOpenBaoValues).
	openbaoServiceAccountNameKey   = "openbao.serviceAccount.name"
	openbaoServiceAccountCreateKey = "openbao.serviceAccount.create"
	openbaoRoleCreateKey           = "openbao.role.create"

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

	if err := setRegistryDefault(values); err != nil {
		return nil, err
	}

	if err := setHostnameValues(values, core.Spec.Hostname); err != nil {
		return nil, err
	}

	if err := setEditionValue(values, core.Spec.Edition); err != nil {
		return nil, err
	}

	if err := setLicenseValues(values, core.Spec.License); err != nil {
		return nil, err
	}

	if err := setPostgreSQLValues(values, core.Spec.PostgreSQL); err != nil {
		return nil, err
	}

	if err := setRedisValues(values, core.Spec.Redis); err != nil {
		return nil, err
	}

	if err := setObjectStorageValues(values, core.Spec.ObjectStorage); err != nil {
		return nil, err
	}

	if err := setOpenBaoValues(values, core.Spec.OpenBao); err != nil {
		return nil, err
	}

	if err := mergeUserValues(values, core); err != nil {
		return nil, err
	}

	return values, setOverrideValues(values)
}

// mergeUserValues merges spec.chart.values over the derived values.
func mergeUserValues(values support.Values, core *apiv2alpha1.GitLabCore) error {
	return release.MergeUserValues(values, core.Spec.Chart.Values.Object)
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

// setRegistryDefault turns the container registry off.
//
// The registry keeps its images in object storage of its own, configured
// through registry.storage, which no structured field covers: the consolidated
// object storage of spec.objectStorage does not reach it. An instance would
// therefore need free-form values to render with a registry that works, so the
// default is off rather than a component that comes up unconfigured.
//
// This is a default rather than an override. An instance that wants a registry
// turns it back on in spec.chart.values, where it also has to supply the
// storage.
func setRegistryDefault(values support.Values) error {
	return errors.Wrapf(values.SetValue(registryEnabledKey, false), "failed to set %s", registryEnabledKey)
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

// setEditionValue maps spec.edition onto the chart edition, which selects the
// image repository every component pulls from.
//
// An empty edition leaves the chart default in place. The field defaults to
// `ee`, so this only happens for an object stored before the field existed.
func setEditionValue(values support.Values, edition apiv2alpha1.Edition) error {
	if edition == "" {
		return nil
	}

	return errors.Wrapf(values.SetValue(editionKey, string(edition)), "failed to set %s", editionKey)
}

// setPostgreSQLValues points the chart at the PostgreSQL server. The chart
// bundles no database since version 10, so these values, or the free-form
// equivalents, are what makes a release render at all.
//
// Only the host and the password Secret are mapped. The port, the database, and
// the user keep their chart defaults, and an instance that needs another one
// sets it in the free-form values.
func setPostgreSQLValues(values support.Values, psql *apiv2alpha1.PostgreSQLSpec) error {
	if psql == nil {
		return nil
	}

	mapping := map[string]interface{}{
		psqlHostKey:           psql.Host,
		psqlPasswordSecretKey: psql.PasswordSecretRef.Name,
		psqlPasswordKeyKey:    psql.PasswordSecretRef.Key,
	}

	for key, value := range mapping {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
	}

	return nil
}

// setRedisValues points the chart at the Redis server, which Valkey can stand
// in for. Like PostgreSQL, the chart bundles none since version 10.
//
// The chart enables Redis authentication by default, so naming the Secret is
// all the structured field has to do.
func setRedisValues(values support.Values, redis *apiv2alpha1.RedisSpec) error {
	if redis == nil {
		return nil
	}

	mapping := map[string]interface{}{
		redisHostKey:       redis.Host,
		redisAuthSecretKey: redis.PasswordSecretRef.Name,
		redisAuthKeyKey:    redis.PasswordSecretRef.Key,
	}

	for key, value := range mapping {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
	}

	return nil
}

// setObjectStorageValues points the consolidated object storage of the chart at
// the Secret that holds the connection.
//
// Naming the Secret also turns the consolidated object storage on: the
// connection configures nothing while it is off, so a resource that carries one
// means to use it. The registry, the Pages daemon, and the backup toolbox read
// settings of their own, which stay in the free-form values.
//
// The Secret is passed through rather than read. It holds the endpoint, the
// region, and the credentials in the format the chart expects, and no
// credential passes through the Operator.
func setObjectStorageValues(values support.Values, storage *apiv2alpha1.ObjectStorageSpec) error {
	if storage == nil {
		return nil
	}

	mapping := map[string]interface{}{
		objectStoreEnabledKey:          true,
		objectStoreConnectionSecretKey: storage.ConnectionSecretRef.Name,
		objectStoreConnectionKeyKey:    storage.ConnectionSecretRef.Key,
	}

	for key, value := range mapping {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
	}

	return nil
}

// setOpenBaoValues points the chart at the OpenBao instance's own PostgreSQL database and turns
// on both the GitLab-side integration and the bundled OpenBao subchart.
//
// Only the host and the password Secret are mapped unconditionally: the port, the database, and
// the username fall back to the chart's own defaults (which the CRD's kubebuilder defaults also
// carry once the API server has applied them) when left unset here, the same way
// setPostgreSQLValues treats the instance's own database.
//
// It also names the ServiceAccount OpenBao's pod runs as, and turns the chart's own
// ServiceAccount and Role off in favor of it: both default on in the chart, and would grant
// get/update/patch on Pods in the namespace through RBAC objects the Operator did not create and
// does not track. The Operator does not manage RBAC on the cluster it reconciles, so that Role
// and its RoleBinding are an administrator prerequisite, the same way the shared secrets
// ServiceAccount is (see setSharedSecretsValues).
func setOpenBaoValues(values support.Values, openbao *apiv2alpha1.OpenBaoSpec) error {
	if openbao == nil {
		return nil
	}

	mapping := map[string]interface{}{
		openbaoEnabledKey:              true,
		openbaoInstallKey:              true,
		openbaoPsqlHostKey:             openbao.PostgreSQL.Host,
		openbaoPsqlPasswordSecretKey:   openbao.PostgreSQL.PasswordSecretRef.Name,
		openbaoPsqlPasswordKeyKey:      openbao.PostgreSQL.PasswordSecretRef.Key,
		openbaoServiceAccountNameKey:   openbao.ServiceAccount.Name,
		openbaoServiceAccountCreateKey: false,
		openbaoRoleCreateKey:           false,
	}

	if openbao.PostgreSQL.Port != 0 {
		mapping[openbaoPsqlPortKey] = openbao.PostgreSQL.Port
	}

	if openbao.PostgreSQL.Database != "" {
		mapping[openbaoPsqlDatabaseKey] = openbao.PostgreSQL.Database
	}

	if openbao.PostgreSQL.Username != "" {
		mapping[openbaoPsqlUsernameKey] = openbao.PostgreSQL.Username
	}

	for key, value := range mapping {
		if err := values.SetValue(key, value); err != nil {
			return errors.Wrapf(err, "failed to set %s", key)
		}
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
