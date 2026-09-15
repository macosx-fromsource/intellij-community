package framework

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// diagnosticKinds are dumped whole, with their status, when a suite fails. They
// are unstructured so that a kind the harness has no Go type for is still
// collected.
var diagnosticKinds = []schema.GroupVersionKind{
	{Group: "apps.gitlab.com", Version: "v2alpha1", Kind: "SiphonList"},
	{Group: "apps.gitlab.com", Version: "v2alpha1", Kind: "GitLabCoreList"},
	{Group: "apps", Version: "v1", Kind: "DeploymentList"},
	{Group: "apps", Version: "v1", Kind: "StatefulSetList"},
	{Group: "", Version: "v1", Kind: "ConfigMapList"},
	{Group: "", Version: "v1", Kind: "ServiceAccountList"},
	{Group: "", Version: "v1", Kind: "PodList"},
}

// diagnosticLogTail bounds the pod log of a failure artifact.
const diagnosticLogTail = int64(400)

// DumpDiagnostics writes what the namespace looked like into the artifacts
// directory.
//
// It runs before the namespace is deleted, which is the whole reason it exists:
// the debug collector of the continuous integration job runs after the test
// binary has exited, by which time the namespace is gone and the only thing left
// to read is a log. This is the artifact that explains a failure.
func (e *Env) DumpDiagnostics(t *testing.T) {
	t.Helper()

	// A context of its own: the one of the suite is cancelled by now.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(e.Ctx), time.Minute)
	defer cancel()

	dir := filepath.Join(e.Config.ArtifactsDir, t.Name())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Logf("could not create the diagnostics directory %s: %v", dir, err)

		return
	}

	t.Logf("writing diagnostics to %s", dir)

	e.dumpObjects(ctx, t, dir)
	e.dumpEvents(ctx, t, dir)
	e.dumpPodLogs(ctx, t, dir)
	e.dumpOperator(ctx, t, dir)
}

func (e *Env) dumpObjects(ctx context.Context, t *testing.T, dir string) {
	t.Helper()

	for _, gvk := range diagnosticKinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)

		if err := e.Client.List(ctx, list, client.InNamespace(e.Namespace)); err != nil {
			// A kind the cluster does not serve is not worth a line of noise.
			continue
		}

		if len(list.Items) == 0 {
			continue
		}

		writeDiagnostic(t, dir, strings.ToLower(gvk.Kind)+".yaml", list)
	}
}

func (e *Env) dumpEvents(ctx context.Context, t *testing.T, dir string) {
	t.Helper()

	events := &corev1.EventList{}
	if err := e.Client.List(ctx, events, client.InNamespace(e.Namespace)); err != nil {
		t.Logf("could not list the events of %s: %v", e.Namespace, err)

		return
	}

	sort.Slice(events.Items, func(i, j int) bool {
		return eventTime(events.Items[i]).Before(eventTime(events.Items[j]))
	})

	var report strings.Builder

	for _, event := range events.Items {
		fmt.Fprintf(&report, "%s\t%s\t%s/%s\t%s\t%s\n",
			eventTime(event).Format(time.RFC3339), event.Type,
			event.InvolvedObject.Kind, event.InvolvedObject.Name,
			event.Reason, event.Message)
	}

	writeDiagnosticText(t, dir, "events.txt", report.String())
}

func (e *Env) dumpPodLogs(ctx context.Context, t *testing.T, dir string) {
	t.Helper()

	pods := &corev1.PodList{}
	if err := e.Client.List(ctx, pods, client.InNamespace(e.Namespace)); err != nil {
		return
	}

	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			logs, err := e.PodLog(ctx, pod.Namespace, pod.Name, container.Name, diagnosticLogTail)
			if err != nil {
				logs = "could not read the log: " + err.Error()
			}

			writeDiagnosticText(t, dir, fmt.Sprintf("pod-%s-%s.log", pod.Name, container.Name), logs)
		}
	}
}

// dumpOperator records the state of the Operator itself, which lives outside the
// namespace of the suite and is therefore the one thing the namespace dump misses.
func (e *Env) dumpOperator(ctx context.Context, t *testing.T, dir string) {
	t.Helper()

	namespace := e.Config.OperatorNamespace

	pods := &corev1.PodList{}
	if err := e.Client.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return
	}

	for _, pod := range pods.Items {
		logs, err := e.PodLog(ctx, pod.Namespace, pod.Name, managerContainer, diagnosticLogTail)
		if err != nil {
			continue
		}

		writeDiagnosticText(t, dir, "operator-"+pod.Name+".log", logs)
	}

	values, err := e.Helm.Values(ctx, operatorRelease, namespace)
	if err == nil {
		writeDiagnostic(t, dir, "operator-values.yaml", values)
	}
}

func writeDiagnostic(t *testing.T, dir, name string, object any) {
	t.Helper()

	encoded, err := yaml.Marshal(object)
	if err != nil {
		t.Logf("could not encode %s: %v", name, err)

		return
	}

	writeDiagnosticText(t, dir, name, string(encoded))
}

func writeDiagnosticText(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Logf("could not write %s: %v", path, err)
	}
}

// eventTime is the best timestamp an event carries. The core API leaves the
// deprecated fields set for a while, so several may be empty.
func eventTime(event corev1.Event) time.Time {
	switch {
	case !event.LastTimestamp.IsZero():
		return event.LastTimestamp.Time
	case !event.EventTime.IsZero():
		return event.EventTime.Time
	case !event.FirstTimestamp.IsZero():
		return event.FirstTimestamp.Time
	default:
		return event.CreationTimestamp.Time
	}
}
