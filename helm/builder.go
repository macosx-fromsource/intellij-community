package helm

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/pkg/errors"

	"helm.sh/helm/v4/pkg/action"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"

	"k8s.io/kubectl/pkg/scheme"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

// Builder provides an interface to build and render a Helm template.
type Builder interface {

	// Chart returns the Helm chart that will be rendered.
	Chart() *chart.Chart

	// Namespace returns namespace of the template.
	Namespace() string

	// SetNamespace sets namespace of the template. Changes will not take effect after rendering the
	// template.
	SetNamespace(namespace string)

	// ReleaseName returns release name of the template.
	ReleaseName() string

	// SetReleaseName sets release name of the template. Changes will not take effect after rendering
	// the template.
	SetReleaseName(releaseName string)

	// HooksDisabled returns true if hooks are disabled for the template.
	HooksDisabled() bool

	// DisableHooks disables hooks for the template. Changes will not take effect after rendering the
	// template.
	DisableHooks()

	// EnableHooks enables hooks for the template. Changes will not take effect after rendering the
	// template.
	EnableHooks()

	// Render renders the template with the provided values and parses the objects.
	Render(values support.Values) (Template, error)
}

// NewBuilder creates a new builder interface for Helm template.
func NewBuilder(charts charts.Catalog) (Builder, error) {
	envSettings := cli.New()
	actionConfig := action.NewConfiguration(
		action.ConfigurationSetLogger(slog.DiscardHandler),
	)

	if err := actionConfig.Init(envSettings.RESTClientGetter(), envSettings.Namespace(), memoryStorageDriver); err != nil {
		return nil, err
	}

	kubeVersion := settings.DefaultKubeVersion
	kubeAPIVersions := settings.DefaultKubeAPIVersions

	if capabilities, err := settings.GetKubeCapabilities(actionConfig); err == nil {
		kubeVersion = &capabilities.KubeVersion
		kubeAPIVersions = capabilities.APIVersions
	}

	client := action.NewInstall(actionConfig)
	client.DryRunStrategy = action.DryRunClient
	client.Replace = true
	client.KubeVersion = kubeVersion
	client.APIVersions = kubeAPIVersions

	chart := charts.First()

	if chart == nil {
		return nil, errors.Errorf("the specified chart not found")
	}

	return &defaultBuilder{
		client:      client,
		chart:       chart,
		namespace:   envSettings.Namespace(),
		releaseName: defaultReleaseName,
	}, nil
}

const (
	defaultReleaseName  = "ephemeral"
	memoryStorageDriver = "memory"
)

type defaultBuilder struct {
	client       *action.Install
	chart        *chart.Chart
	namespace    string
	releaseName  string
	disableHooks bool
}

func (b *defaultBuilder) Chart() *chart.Chart {
	return b.chart
}

// Namespace returns namespace of the template.
func (b *defaultBuilder) Namespace() string {
	return b.namespace
}

// SetNamespace sets namespace of the template.
func (b *defaultBuilder) SetNamespace(namespace string) {
	b.namespace = namespace
}

// ReleaseName returns release name of the template.
func (b *defaultBuilder) ReleaseName() string {
	return b.releaseName
}

// SetReleaseName sets release name of the template.
func (b *defaultBuilder) SetReleaseName(releaseName string) {
	b.releaseName = releaseName
}

// HooksDisabled returns true if hooks are disabled in the template.
func (b *defaultBuilder) HooksDisabled() bool {
	return b.disableHooks
}

// DisableHooks disables hooks for the template.
func (b *defaultBuilder) DisableHooks() {
	b.disableHooks = true
}

// EnableHooks enables hooks for the template.
func (b *defaultBuilder) EnableHooks() {
	b.disableHooks = false
}

// Render renders the template with the provided values and parses the objects.
func (b *defaultBuilder) Render(values support.Values) (Template, error) {
	b.client.DisableHooks = b.disableHooks
	b.client.Namespace = b.namespace
	b.client.ReleaseName = b.releaseName

	releaser, err := b.client.Run(b.chart, values)
	if err != nil {
		return nil, err
	}

	release, ok := releaser.(*releasev1.Release)
	if !ok {
		return nil, errors.New("failed to assert type of helm release")
	}

	manifests := releaseutil.SplitManifests(release.Manifest)

	if !b.disableHooks {
		for index, hook := range release.Hooks {
			manifests[fmt.Sprintf("hook-%d", index)] =
				fmt.Sprintf("# Hook: %s\n%s\n", hook.Path, hook.Manifest)
		}
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	template := newMutableTemplate(b.releaseName, b.namespace)

	for _, yaml := range manifests {
		// Skip empty YAML documents (can occur when feature flags disable certain components)
		if isEmptyYAML(yaml) {
			continue
		}

		obj, _, err := decode([]byte(yaml), nil, nil)
		if err != nil {
			template.warnings = append(template.warnings, err)
		} else {
			template.objects = append(template.objects, obj)
		}
	}

	return template, nil
}

// isEmptyYAML checks if a YAML string is empty or contains only whitespace and comments.
// This is used to skip empty manifests that occur when feature flags disable certain components.
func isEmptyYAML(yaml string) bool {
	trimmed := strings.TrimSpace(yaml)
	if trimmed == "" {
		return true
	}

	// Check if the YAML only contains comments and whitespace
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") && line != "---" {
			return false
		}
	}

	return true
}
