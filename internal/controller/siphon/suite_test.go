package siphon

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"helm.sh/helm/v4/pkg/chart/common"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/capabilities"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts/populate"
)

// testSecretKey is the key every test Secret holds its credential under.
const testSecretKey = "password"

const (
	testName      = "siphon"
	testNamespace = "gitlab-system"

	testGitLabName         = "gitlab"
	testGitLabVersion      = "19.2.0"
	testGitLabChartVersion = "10.2.0"

	testSourceHost = "gitlab-postgresql-rw.databases.svc.cluster.local"
	testQueueURL   = "nats://nats.nats.svc.cluster.local:4222"
	testSinkHost   = "clickhouse.databases.svc.cluster.local"
	testSinkDB     = "gitlab_clickhouse_main_production"
	testSinkUser   = "gitlab"
)

func TestMain(m *testing.M) {
	settings.Load()

	// The catalogue answers two questions offline: which chart versions the
	// Operator carries, for the error that reports an unusable one, and which
	// application version a GitLab chart version deploys.
	_ = charts.PopulateGlobalCatalog(populate.WithSearchPath(settings.HelmChartsDirectory))

	utilruntime.Must(apiv2alpha1.AddToScheme(scheme.Scheme))

	os.Exit(m.Run())
}

// newSiphon builds a Siphon with every required field set, which is what the
// values layer and the renderer are exercised against.
func newSiphon(options ...func(*apiv2alpha1.Siphon)) *apiv2alpha1.Siphon {
	siphon := &apiv2alpha1.Siphon{
		ObjectMeta: metav1.ObjectMeta{Name: testName, Namespace: testNamespace},
		Spec: apiv2alpha1.SiphonSpec{
			GitLabRef: apiv2alpha1.GitLabReference{Name: testGitLabName},
			Source: apiv2alpha1.PostgreSQLSourceSpec{
				Host:              testSourceHost,
				Port:              5432,
				Database:          "gitlabhq_production",
				User:              "siphon",
				SSLMode:           "require",
				AdvisoryLockID:    1,
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "siphon-postgresql", Key: testSecretKey},
			},
			Queue: apiv2alpha1.QueueSpec{URL: testQueueURL},
			Sink: apiv2alpha1.ClickHouseSinkSpec{
				Host:              testSinkHost,
				Port:              9000,
				Database:          testSinkDB,
				Username:          testSinkUser,
				PasswordSecretRef: apiv2alpha1.SecretKeySelector{Name: "clickhouse-gitlab", Key: testSecretKey},
			},
			Tables: apiv2alpha1.TablesSpec{Source: apiv2alpha1.TablesSourceAuto},
			Chart:  apiv2alpha1.ChartSpec{Version: chartVersion()},
		},
	}

	for _, option := range options {
		option(siphon)
	}

	return siphon
}

// testRelease is what the reconciler resolves before it derives any values.
func testRelease() Release {
	return Release{
		GitLabVersion: testGitLabVersion,
		TablesImage:   tablesImageRepository + ":v" + testGitLabVersion + "-ee",
		TablesSource:  apiv2alpha1.TablesSourceImageVolume,
	}
}

// chartVersion is the Siphon chart version the tests render, from the pin the
// Operator carries.
func chartVersion() string {
	versions := charts.GlobalCatalog().Versions(chartName)
	if len(versions) == 0 {
		return ""
	}

	return versions[0]
}

// chartsDirectory is where the retrieved charts live.
func chartsDirectory() string {
	return settings.HelmChartsDirectory
}

// requireChart skips a test that needs the Siphon chart archive, which
// `task retrieve-charts` fetches.
func requireChart(t *testing.T) {
	t.Helper()

	if chartVersion() == "" {
		t.Skipf("no Siphon chart in %s; run `task retrieve-charts`", chartsDirectory())
	}
}

// kubeVersion builds the version a cluster reports.
func kubeVersion(version, major, minor string) *common.KubeVersion {
	return &common.KubeVersion{Version: version, Major: major, Minor: minor}
}

// discoveredCapabilities is what a cluster new enough to mount an image volume,
// serving the given group versions, discovers as.
func discoveredCapabilities(apiVersions ...string) *capabilities.Capabilities {
	return &capabilities.Capabilities{
		KubeVersion: kubeVersion("v1.35.0", "1", "35"),
		APIVersions: apiVersions,
	}
}

// availableGitLab builds a GitLabCore that is available and reports the given
// application version, which is what a Siphon waits for. An empty version stands
// for an instance that has not published one yet.
func availableGitLab(gitlabVersion string) *apiv2alpha1.GitLabCore {
	core := &apiv2alpha1.GitLabCore{
		ObjectMeta: metav1.ObjectMeta{Name: testGitLabName, Namespace: testNamespace},
		Status: apiv2alpha1.GitLabCoreStatus{
			Version:       testGitLabChartVersion,
			GitLabVersion: gitlabVersion,
			Phase:         "Running",
		},
	}

	apimeta.SetStatusCondition(&core.Status.Conditions, metav1.Condition{
		Type:   gitLabConditionAvailable,
		Status: metav1.ConditionTrue,
		Reason: "WorkloadsReady",
	})

	return core
}

// testReconciler builds a reconciler over a fake client.
func testReconciler(t *testing.T, objects ...client.Object) *Reconciler {
	t.Helper()

	mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), apimeta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), apimeta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ServiceAccount"), apimeta.RESTScopeNamespace)
	mapper.Add(appsv1.SchemeGroupVersion.WithKind("Deployment"), apimeta.RESTScopeNamespace)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithRESTMapper(mapper).
		WithObjects(objects...).
		WithStatusSubresource(&apiv2alpha1.Siphon{}).
		WithReturnManagedFields().
		Build()

	return &Reconciler{
		Client:    fakeClient,
		Log:       testLogger(t),
		Scheme:    scheme.Scheme,
		Discovery: fakeDiscovery(t, "v1.35.0", "1", "35"),
		Recorder:  &recordingRecorder{},
	}
}

// fakeDiscovery answers as a cluster of the given version that serves the group
// versions the chart branches on. The reconciler discovers on every pass, both to
// render against the cluster and to resolve the tables source.
func fakeDiscovery(t *testing.T, version, major, minor string) discovery.DiscoveryInterface {
	t.Helper()

	discoveryClient, ok := fakeclientset.NewSimpleClientset().Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok)

	discoveryClient.FakedServerVersion = &apiversion.Info{
		GitVersion: version, Major: major, Minor: minor,
	}
	discoveryClient.Resources = []*metav1.APIResourceList{{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{{Kind: "Deployment", Name: "deployments"}},
	}}

	return discoveryClient
}

// testLogger returns a logger that routes into the test output.
func testLogger(t *testing.T) logr.Logger {
	t.Helper()

	return testr.New(t)
}

// recordingRecorder captures the events a reconcile emits, so a test can assert
// on what an administrator would see.
type recordingRecorder struct {
	events []recordedEvent
}

// recordedEvent is one captured event.
type recordedEvent struct {
	reason  string
	action  string
	message string
}

func (r *recordingRecorder) Eventf(_ runtime.Object, _ runtime.Object, _, reason, action, note string, args ...interface{}) {
	r.events = append(r.events, recordedEvent{
		reason:  reason,
		action:  action,
		message: fmt.Sprintf(note, args...),
	})
}
