// Package render templates Helm charts into Kubernetes objects without any
// cluster interaction or release machinery.
//
// Use it for the v2-era resources; the legacy helm/ package renders the
// v1beta1 GitLab controller and is frozen. Every manifest is returned as
// *unstructured.Unstructured, so third-party custom resources survive
// rendering like built-in kinds. The sibling objects subpackage selects and
// mutates the rendered slices.
//
// Every render is composed as revision 1 of an install; express upgrade
// behavior through hook events, which Result.Hooks models.
//
// # Offline rendering caveats
//
// The legacy helm/ package renders with a client-side dry run and shares
// every caveat below.
//
//   - The lookup template function always returns an empty map, without an
//     error and without a warning, so a template branching on an existing
//     object takes the "not found" path. Read cluster state in the caller and
//     pass it in through Request.Values.
//   - Rendered objects carry no release ownership metadata. Helm stamps the
//     app.kubernetes.io/managed-by label and the meta.helm.sh/release-name
//     and meta.helm.sh/release-namespace annotations on apply, not on render.
//   - Capabilities come only from the Request. Read them from a cluster with
//     the render/capabilities package.
//   - DNS is disabled, so getHostByName returns an empty string, and
//     rendering is not strict, so a missing value renders as "<no value>".
//     Both match the Helm CLI defaults.
//   - Hooks, CRDs, and NOTES.txt are returned as data and never executed,
//     applied, or printed. There is no release history, so no rollback, no
//     resource adoption, and no post-renderer support.
//   - Values are validated against the values.schema.json files of the chart
//     and its dependencies, so a schema violation fails the render.
package render

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"helm.sh/helm/v4/pkg/chart/common"
	commonutil "helm.sh/helm/v4/pkg/chart/common/util"
	chartloader "helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartv2util "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/engine"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	releaseutil "helm.sh/helm/v4/pkg/release/v1/util"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation"
	k8syaml "sigs.k8s.io/yaml"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// defaultNamespace is used when the request does not specify a namespace,
// matching the Helm CLI default.
const defaultNamespace = "default"

// notesFileName is the chart template whose rendered output documents the
// release for the user instead of being applied.
const notesFileName = "NOTES.txt"

// Labels that Render stamps on every object and hook. Use them to find the
// output of one release again, above all its cluster-scoped objects, which
// cannot hold an owner reference to a namespaced custom resource:
//
//	client.MatchingLabels{render.ReleaseNameLabel: name}
const (
	// ReleaseNameLabel names the release an object was rendered for.
	ReleaseNameLabel = "operator.gitlab.com/release-name"

	// ReleaseNamespaceLabel names the namespace of that release.
	ReleaseNamespaceLabel = "operator.gitlab.com/release-namespace"
)

// Request describes a single chart-templating operation. It is a plain value:
// construct a new Request for every render instead of mutating a shared one.
type Request struct {
	// ChartPath points at a packaged chart (.tgz) or an unpacked chart
	// directory.
	ChartPath string

	// ReleaseName becomes .Release.Name in the templates. Required.
	ReleaseName string

	// Namespace becomes .Release.Namespace in the templates. Defaults to
	// "default". The renderer does not stamp it onto rendered objects;
	// only templates that reference .Release.Namespace carry it.
	Namespace string

	// Values are the user-supplied chart values, merged over the chart
	// defaults.
	Values support.Values

	// KubeVersion sets .Capabilities.KubeVersion. When nil, the Helm SDK
	// default applies.
	KubeVersion *common.KubeVersion

	// APIVersions sets .Capabilities.APIVersions. A non-empty set replaces
	// the Helm default set instead of extending it, so pass a complete one:
	// .Capabilities.APIVersions.Has reports false for every entry left out.
	// Entries take the group-version form ("policy/v1") or the
	// group-version-kind form ("policy/v1/PodDisruptionBudget"), and charts
	// probe both. Read a complete set from a cluster with the
	// render/capabilities package. When empty, the Helm SDK default set
	// applies, which enumerates the client-go scheme rather than a cluster.
	APIVersions []string
}

// Result holds the outcome of rendering a chart.
type Result struct {
	// Objects are the rendered non-hook manifests in deterministic order:
	// sorted by kind following the Helm install order, with the rendering
	// order (template path, then in-file position) preserved within a
	// kind.
	Objects []*unstructured.Unstructured

	// Hooks are the manifests that declare a "helm.sh/hook" annotation.
	// They never appear in Objects; callers decide whether and how to run
	// them. The "helm.sh/hook-output-log-policy" annotation is not modeled.
	Hooks []Hook

	// CRDs are the definitions the chart ships in its crds/ directories,
	// including those of its dependencies, in the chart file order. Apply
	// them before the objects; Render never does.
	CRDs []*unstructured.Unstructured

	// Notes is the rendered NOTES.txt of the top-level chart, the text the
	// Helm CLI prints after an install. It is documentation, never applied.
	// Notes of dependency charts are excluded, like the Helm default.
	Notes string

	// Warnings reports documents that were dropped because they could not
	// be parsed as Kubernetes objects or carried unusable hook
	// annotations.
	Warnings []Warning

	// AppVersion is the appVersion the top-level chart declares, the version
	// of the application it deploys, verbatim and so possibly prefixed with
	// a "v". It is unrelated to the chart version, and a chart may declare
	// none, in which case it is empty.
	//
	// Read it to answer what a render deploys without reaching for a chart
	// catalog: it comes from the chart that was actually rendered, whether
	// that came from disk or from a repository.
	AppVersion string
}

// Hook is a rendered manifest that declares a "helm.sh/hook" annotation.
type Hook struct {
	// Path is the chart-relative template path the hook was rendered
	// from.
	Path string

	// Events lists the hook events, trimmed and lowercased, e.g.
	// "pre-install".
	Events []string

	// Weight is the "helm.sh/hook-weight" annotation value. A missing or
	// unparseable annotation yields 0, matching the Helm SDK behavior.
	Weight int

	// DeletePolicies lists the "helm.sh/hook-delete-policy" annotation
	// values, trimmed and lowercased. The canonical values are
	// "hook-succeeded", "hook-failed", and "before-hook-creation".
	// Interpreting them is the caller's concern: the Helm SDK treats a
	// missing policy as "before-hook-creation" at execution time.
	DeletePolicies []string

	// Object is the decoded hook manifest.
	Object *unstructured.Unstructured
}

// HasEvent reports whether the hook fires for the given event.
func (h Hook) HasEvent(event string) bool {
	return slices.Contains(h.Events, event)
}

// HooksFor returns the hooks that fire for the given event, in the Helm
// execution order: ascending weight, then object name. It returns a fresh
// slice and leaves Result.Hooks untouched, but the copies are shallow: the
// returned hooks share their Object with Result.Hooks.
func (r *Result) HooksFor(event string) []Hook {
	var hooks []Hook

	for _, hook := range r.Hooks {
		if hook.HasEvent(event) {
			hooks = append(hooks, hook)
		}
	}

	sort.SliceStable(hooks, func(i, j int) bool {
		if hooks[i].Weight == hooks[j].Weight {
			return hooks[i].Object.GetName() < hooks[j].Object.GetName()
		}

		return hooks[i].Weight < hooks[j].Weight
	})

	return hooks
}

// WarningReason classifies why a rendered document was dropped.
type WarningReason string

// Warning reasons.
const (
	// WarningUnparseableDocument marks a document that is not valid YAML.
	WarningUnparseableDocument WarningReason = "UnparseableDocument"

	// WarningMissingTypeMeta marks a document without apiVersion or kind.
	WarningMissingTypeMeta WarningReason = "MissingTypeMeta"

	// WarningInvalidHook marks a manifest whose "helm.sh/hook" annotation
	// is not usable: an unknown hook event or a non-string value.
	WarningInvalidHook WarningReason = "InvalidHook"
)

// Warning describes a rendered document that was dropped from the result.
type Warning struct {
	// Path is the chart-relative template path the document was rendered
	// from.
	Path string

	// Reason classifies the warning.
	Reason WarningReason

	// Err carries the underlying parse error, when there is one.
	Err error
}

// String renders the warning for logging.
func (w Warning) String() string {
	if w.Err != nil {
		return fmt.Sprintf("%s: %s: %v", w.Path, w.Reason, w.Err)
	}

	return fmt.Sprintf("%s: %s", w.Path, w.Reason)
}

// Render templates the chart described by the request and parses the output
// into unstructured objects. It performs pure, offline templating: no cluster
// connection, no release records, and no hook execution.
func Render(request Request) (*Result, error) {
	if request.ChartPath == "" {
		return nil, fmt.Errorf("render: ChartPath is required")
	}

	if request.ReleaseName == "" {
		return nil, fmt.Errorf("render: ReleaseName is required")
	}

	loadedChart, err := loadChart(request.ChartPath)
	if err != nil {
		return nil, err
	}

	values := map[string]interface{}(request.Values)
	if values == nil {
		values = map[string]interface{}{}
	}

	if err := chartv2util.ProcessDependencies(loadedChart, values); err != nil {
		return nil, fmt.Errorf("render: processing dependencies of chart %q: %w", loadedChart.Name(), err)
	}

	namespace := request.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	options := common.ReleaseOptions{
		Name:      request.ReleaseName,
		Namespace: namespace,
		Revision:  1,
		IsInstall: true,
	}

	renderValues, err := commonutil.ToRenderValues(loadedChart, values, options, capabilities(request))
	if err != nil {
		return nil, fmt.Errorf("render: composing values for chart %q: %w", loadedChart.Name(), err)
	}

	files, err := engine.Render(loadedChart, renderValues)
	if err != nil {
		return nil, fmt.Errorf("render: templating chart %q: %w", loadedChart.Name(), err)
	}

	result := &Result{}

	if loadedChart.Metadata != nil {
		result.AppVersion = loadedChart.Metadata.AppVersion
	}

	parseFiles(result, files, loadedChart.Name())
	parseCRDs(result, loadedChart)
	sortObjectsByKind(result.Objects)
	sortHooksByKind(result.Hooks)

	if err := stampReleaseLabels(result, request.ReleaseName, namespace); err != nil {
		return nil, err
	}

	return result, nil
}

// stampReleaseLabels marks every rendered object and hook with the release
// identity. Definitions are left alone: they are installed once, never
// updated, and commonly shared between releases.
func stampReleaseLabels(result *Result, release, namespace string) error {
	labels := map[string]string{
		ReleaseNameLabel:      release,
		ReleaseNamespaceLabel: namespace,
	}

	for key, value := range labels {
		// Fail here rather than on every apply.
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("render: %s cannot hold %q: %s", key, value, strings.Join(errs, "; "))
		}
	}

	for _, obj := range result.Objects {
		if err := stampLabels(obj, labels); err != nil {
			return err
		}
	}

	for _, hook := range result.Hooks {
		if err := stampLabels(hook.Object, labels); err != nil {
			return err
		}
	}

	return nil
}

// stampLabels merges the labels into metadata.labels of one object. It
// touches nothing else: a workload selector is immutable after creation, and
// a change to the pod template labels rolls every pod.
func stampLabels(obj *unstructured.Unstructured, labels map[string]string) error {
	existing, _, err := unstructured.NestedStringMap(obj.Object, "metadata", "labels")
	if err != nil {
		return fmt.Errorf("render: reading labels of %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	if existing == nil {
		existing = map[string]string{}
	}

	maps.Copy(existing, labels)

	if err := unstructured.SetNestedStringMap(obj.Object, existing, "metadata", "labels"); err != nil {
		return fmt.Errorf("render: labelling %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	return nil
}

// parseCRDs decodes the files of the crds/ directories of the chart and its
// dependencies. Hook annotations have no effect there, so every document
// becomes a CRD entry.
func parseCRDs(result *Result, loadedChart *chartv2.Chart) {
	for _, crd := range loadedChart.CRDObjects() {
		for _, document := range splitDocuments(string(crd.File.Data)) {
			if isEmptyDocument(document) {
				continue
			}

			obj, warning := decodeDocument(crd.Filename, document)
			if warning != nil {
				result.Warnings = append(result.Warnings, *warning)
				continue
			}

			result.CRDs = append(result.CRDs, obj)
		}
	}
}

// loadChart loads a packaged or unpacked chart from disk and asserts that it
// uses chart API v2, the only version the public Helm SDK exposes.
func loadChart(chartPath string) (*chartv2.Chart, error) {
	charter, err := chartloader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("render: loading chart from %q: %w", chartPath, err)
	}

	loadedChart, ok := charter.(*chartv2.Chart)
	if !ok {
		return nil, fmt.Errorf("render: chart at %q uses an unsupported chart API version (only v2 is supported)", chartPath)
	}

	return loadedChart, nil
}

// capabilities builds the .Capabilities render context from the request,
// falling back to the Helm SDK defaults. The request is the only source:
// composing a cluster reading with an operator-wide override is a policy
// decision that belongs to the caller holding both.
//
// A supplied API version set replaces the defaults rather than extending
// them, so that a set can state that a cluster does not serve a group
// version. Merging would reinstate the client-go scheme entries, such as
// extensions/v1beta1, that no current cluster serves.
//
// caps.APIVersions aliases either common.DefaultVersionSet or the slice of
// the caller. Do not write to it; the render context only reads it through
// VersionSet.Has.
func capabilities(request Request) *common.Capabilities {
	caps := *common.DefaultCapabilities

	if request.KubeVersion != nil {
		caps.KubeVersion = *request.KubeVersion
	}

	if len(request.APIVersions) > 0 {
		caps.APIVersions = common.VersionSet(request.APIVersions)
	}

	return &caps
}

// parseFiles splits the rendered files into YAML documents and decodes them
// into the result, separating notes and hooks from regular objects. Files are
// visited in sorted path order and documents in in-file order, so the pre-sort
// object order is deterministic.
func parseFiles(result *Result, files map[string]string, chartName string) {
	paths := make([]string, 0, len(files))

	for filePath := range files {
		paths = append(paths, filePath)
	}

	sort.Strings(paths)

	notesPath := path.Join(chartName, "templates", notesFileName)

	for _, filePath := range paths {
		base := path.Base(filePath)

		if strings.HasPrefix(base, "_") {
			continue
		}

		// Notes are documentation for the user, not manifests to apply. Only
		// the top-level chart contributes them, like the Helm default.
		if strings.HasSuffix(base, notesFileName) {
			if filePath == notesPath {
				result.Notes = files[filePath]
			}

			continue
		}

		parseDocuments(result, filePath, files[filePath])
	}
}

// parseDocuments decodes every YAML document of a single rendered file.
func parseDocuments(result *Result, filePath, content string) {
	for _, document := range splitDocuments(content) {
		if isEmptyDocument(document) {
			continue
		}

		obj, warning := decodeDocument(filePath, document)
		if warning != nil {
			result.Warnings = append(result.Warnings, *warning)
			continue
		}

		hook, isHook, warning := hookFor(filePath, obj)
		if warning != nil {
			result.Warnings = append(result.Warnings, *warning)
			continue
		}

		if isHook {
			result.Hooks = append(result.Hooks, hook)
			continue
		}

		result.Objects = append(result.Objects, obj)
	}
}

// splitDocuments splits a rendered file into its YAML documents, preserving
// the in-file order.
func splitDocuments(content string) []string {
	documents := releaseutil.SplitManifests(content)
	keys := make([]string, 0, len(documents))

	for key := range documents {
		keys = append(keys, key)
	}

	sort.Sort(releaseutil.BySplitManifestsOrder(keys))

	ordered := make([]string, 0, len(keys))

	for _, key := range keys {
		ordered = append(ordered, documents[key])
	}

	return ordered
}

// decodeDocument decodes one YAML document into an unstructured object,
// returning a warning instead of an object when it cannot represent a
// Kubernetes resource.
//
// Keep the detour through the apimachinery JSON decoder. It converts integral
// numbers to int64 and leaves fractions as float64, the shape the unstructured
// accessors expect; sigs.k8s.io/yaml alone yields float64 and makes a rendered
// "replicas: 1" unreadable through NestedInt64. Decoding straight into
// unstructured.Unstructured converts the same way but also requires apiVersion
// and kind, which would collapse the two warning reasons below into one.
func decodeDocument(filePath, document string) (*unstructured.Unstructured, *Warning) {
	jsonBytes, err := k8syaml.YAMLToJSON([]byte(document))
	if err != nil {
		return nil, &Warning{Path: filePath, Reason: WarningUnparseableDocument, Err: err}
	}

	content := map[string]interface{}{}

	if err := utiljson.Unmarshal(jsonBytes, &content); err != nil {
		return nil, &Warning{Path: filePath, Reason: WarningUnparseableDocument, Err: err}
	}

	obj := &unstructured.Unstructured{Object: content}

	if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
		return nil, &Warning{Path: filePath, Reason: WarningMissingTypeMeta}
	}

	return obj, nil
}

// The hook events the Helm SDK recognizes.
const (
	hookEventPreInstall   = "pre-install"
	hookEventPostInstall  = "post-install"
	hookEventPreDelete    = "pre-delete"
	hookEventPostDelete   = "post-delete"
	hookEventPreUpgrade   = "pre-upgrade"
	hookEventPostUpgrade  = "post-upgrade"
	hookEventPreRollback  = "pre-rollback"
	hookEventPostRollback = "post-rollback"
	hookEventTest         = "test"
)

// knownHookEvents are the hook events the Helm SDK recognizes, after the
// legacy "test-success" alias resolved to "test".
var knownHookEvents = map[string]bool{
	hookEventPreInstall:   true,
	hookEventPostInstall:  true,
	hookEventPreDelete:    true,
	hookEventPostDelete:   true,
	hookEventPreUpgrade:   true,
	hookEventPostUpgrade:  true,
	hookEventPreRollback:  true,
	hookEventPostRollback: true,
	hookEventTest:         true,
}

// hookFor builds a Hook from an object that declares a "helm.sh/hook"
// annotation, reporting whether the object is a hook at all. An unusable
// annotation drops the manifest with a warning, like the Helm SDK.
func hookFor(filePath string, obj *unstructured.Unstructured) (Hook, bool, *Warning) {
	annotations := rawAnnotations(obj)

	value, found := annotations[releasev1.HookAnnotation]
	if !found {
		return Hook{}, false, nil
	}

	events, ok := value.(string)
	if !ok {
		return Hook{}, false, &Warning{
			Path:   filePath,
			Reason: WarningInvalidHook,
			Err:    fmt.Errorf("%q annotation is not a string", releasev1.HookAnnotation),
		}
	}

	canonical, unknown := hookEvents(events)
	if unknown != "" {
		return Hook{}, false, &Warning{
			Path:   filePath,
			Reason: WarningInvalidHook,
			Err:    fmt.Errorf("unknown hook event %q", unknown),
		}
	}

	return Hook{
		Path:           filePath,
		Events:         canonical,
		Weight:         hookWeight(annotations[releasev1.HookWeightAnnotation]),
		DeletePolicies: deletePolicies(annotations[releasev1.HookDeleteAnnotation]),
		Object:         obj,
	}, true, nil
}

// rawAnnotations returns metadata.annotations without requiring every value
// to be a string, unlike GetAnnotations, so one non-string value cannot hide
// the hook annotations.
func rawAnnotations(obj *unstructured.Unstructured) map[string]interface{} {
	annotations, _, _ := unstructured.NestedFieldNoCopy(obj.Object, "metadata", "annotations")
	values, _ := annotations.(map[string]interface{})

	return values
}

// hookEvents normalizes the comma-separated hook events like the Helm SDK
// does: trimmed, lowercased, and with the legacy "test-success" alias
// resolved to "test". It reports the first unrecognized event, which the SDK
// treats as reason to drop the whole manifest.
func hookEvents(value string) (events []string, unknown string) {
	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(strings.TrimSpace(item))

		if item == "test-success" {
			item = hookEventTest
		}

		if !knownHookEvents[item] {
			return nil, item
		}

		events = append(events, item)
	}

	return events, ""
}

// deletePolicies splits a "helm.sh/hook-delete-policy" annotation into its
// trimmed, lowercased values, matching the Helm SDK normalization of case and
// whitespace. A non-string annotation value yields no policies.
//
// Unlike the SDK, empty values are dropped, so an annotation rendering to the
// empty string comes back nil and an executor applies the documented default,
// before-hook-creation. The SDK keeps a one-element list holding the empty
// string, which suppresses its default and never deletes such a hook.
func deletePolicies(value interface{}) []string {
	text, ok := value.(string)
	if !ok {
		return nil
	}

	var policies []string

	for _, item := range strings.Split(text, ",") {
		if item = strings.ToLower(strings.TrimSpace(item)); item != "" {
			policies = append(policies, item)
		}
	}

	return policies
}

// hookWeight parses a "helm.sh/hook-weight" annotation value. A missing or
// unparseable value yields 0, matching the Helm SDK. Unlike the SDK, which
// fails the render, an unquoted integer is accepted; the int64 case relies on
// decodeDocument converting integral JSON numbers to int64.
func hookWeight(value interface{}) int {
	switch typed := value.(type) {
	case string:
		weight, err := strconv.Atoi(typed)
		if err != nil {
			return 0
		}

		return weight
	case int64:
		return int(typed)
	default:
		return 0
	}
}

// isEmptyDocument reports whether a YAML document contains only whitespace,
// comments, and separators, which happens when values disable a template.
func isEmptyDocument(document string) bool {
	for line := range strings.SplitSeq(document, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "---" && !strings.HasPrefix(line, "#") {
			return false
		}
	}

	return true
}

// sortObjectsByKind stably sorts the objects by kind following the Helm
// install order. Unknown kinds sort after known ones, alphabetically, and
// objects of the same kind keep their rendering order.
func sortObjectsByKind(objects []*unstructured.Unstructured) {
	rank := installOrderRank()

	sort.SliceStable(objects, func(i, j int) bool {
		return lessByKind(objects[i].GetKind(), objects[j].GetKind(), rank)
	})
}

// sortHooksByKind stably sorts the hooks by the kind of their object, exactly
// like sortObjectsByKind, mirroring how the Helm SDK stores release hooks.
func sortHooksByKind(hooks []Hook) {
	rank := installOrderRank()

	sort.SliceStable(hooks, func(i, j int) bool {
		return lessByKind(hooks[i].Object.GetKind(), hooks[j].Object.GetKind(), rank)
	})
}

// installOrderRank maps every kind of the Helm install order to its position.
func installOrderRank() map[string]int {
	rank := make(map[string]int, len(releaseutil.InstallOrder))

	for position, kind := range releaseutil.InstallOrder {
		rank[kind] = position
	}

	return rank
}

// lessByKind mirrors the ordering semantics of the Helm kind sorter.
func lessByKind(kindA, kindB string, rank map[string]int) bool {
	rankA, knownA := rank[kindA]
	rankB, knownB := rank[kindB]

	switch {
	case !knownA && !knownB:
		return kindA < kindB
	case !knownA:
		return false
	case !knownB:
		return true
	default:
		return rankA < rankB
	}
}
