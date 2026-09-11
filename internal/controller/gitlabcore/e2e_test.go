//go:build e2e

// This end-to-end test drives the GitLabCore reconciler against a real
// cluster: it creates a resource, calls Reconcile the way the manager does,
// and asserts on what reaches the cluster. It is the counterpart of
// internal/render/hookexec/e2e_test.go, which exercises the rendering and hook
// packages the reconciler is built from.
//
// Run it with a kubeconfig pointing at a throwaway cluster that has the
// v2alpha1 definitions installed:
//
//	task install_v2alpha1_crds
//	HELM_CHARTS=$PWD/charts CHART_VERSION=$(head -n1 CHART_VERSIONS) \
//	go test -tags e2e -count=1 -timeout 30m -v -run TestGitLabCoreReconciler ./internal/controller/gitlabcore/
//
// The release renders into the test namespace and nowhere else: Prometheus and
// the Gateway API are off through the values, and cert-manager, the GitLab
// Runner, and every bundled ingress controller through the Operator overrides,
// so every object is namespaced and of a built-in kind. The pods never become
// ready, because there is no PostgreSQL, Redis, or object storage, which the
// test asserts rather than waits for.
//
// | Variable | Description |
// | E2E_KEEP_NAMESPACE=1 | Keeps the namespace for inspection. |

package gitlabcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/events"
	kubectlscheme "k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// e2eHookTimeout bounds a single chart hook. The shared secrets Job pulls the
// toolbox image on a cold cluster, which dominates the wall time of the test.
const e2eHookTimeout = 10 * time.Minute

// e2eGarbageCollectionTimeout bounds the wait for Kubernetes to collect the
// objects the deleted resource owned.
const e2eGarbageCollectionTimeout = 3 * time.Minute

// e2eValues render the whole chart into one namespace, without pointing at real
// infrastructure. Everything that writes outside the namespace is off, so the
// default run touches nothing else in the cluster. Keep the object storage
// entries: the chart asserts them from NOTES.txt and fails the render without
// them.
//
// cert-manager and the GitLab Runner are absent on purpose: the Operator
// overrides both off, whatever the values say.
const e2eValues = `
certmanager-issuer:
  email: admin@example.com
prometheus:
  install: false
registry:
  storage:
    secret: e2e-registry-storage
    key: config
gitlab:
  toolbox:
    backups:
      objectStorage:
        config:
          secret: e2e-backup-storage
          key: config
global:
  gatewayApi:
    enabled: false
    configureCertmanager: false
  ingress:
    enabled: true
    provider: nginx
    class: nginx
    configureCertmanager: false
    tls:
      enabled: false
  redis:
    host: redis.example.com
  psql:
    host: psql.example.com
    password:
      secret: e2e-psql-password
      key: password
  pages:
    objectStore:
      connection:
        secret: e2e-object-storage
        key: connection
  appConfig:
    object_store:
      enabled: true
      connection:
        secret: e2e-object-storage
        key: connection
`

// e2eClusterScopedKinds are the cluster-scoped kinds a release can carry. The
// Operator overrides leave none of them rendered today, so the assertions on
// them hold vacuously; they stay as the guard that catches a release which
// starts writing outside its namespace again.
//
// GatewayClass is the only one the finalizer's sweepUnowned actually reaches
// for, by the release labels; it is a fixed target, not one discovered by
// rendering the release, so nothing here sweeps the other three. That is the
// point of keeping them in this list rather than dropping them: if the chart
// ever renders one of them again, this is what catches the leak, since the
// finalizer no longer discovers cluster-scoped kinds generically. Cleanup
// sweeps every kind in this list after a run that failed before the deletion.
var e2eClusterScopedKinds = []schema.GroupVersionKind{
	gatewayClassGVK,
	{Group: "networking.k8s.io", Version: "v1", Kind: "IngressClass"},
	{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
	{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"},
}

// e2eSkippedKinds are the kinds the reconciler never applies: the definitions
// and the RBAC of the chart.
var e2eSkippedKinds = []schema.GroupVersionKind{
	{Group: "apiextensions.k8s.io", Version: "v1", Kind: crdKind},
	{Group: rbacGroup, Version: "v1", Kind: "ClusterRole"},
	{Group: rbacGroup, Version: "v1", Kind: "ClusterRoleBinding"},
	{Group: rbacGroup, Version: "v1", Kind: "Role"},
	{Group: rbacGroup, Version: "v1", Kind: "RoleBinding"},
}

func TestGitLabCoreReconciler(t *testing.T) {
	settings.Load()

	ctx := context.Background()

	e2eRequireChart(t)

	kubeClient := e2eClient(t)
	e2eRequireDefinition(ctx, t, kubeClient)

	// A random suffix keeps concurrent runs, and reruns within the same second,
	// from colliding on the namespace or on the cluster-scoped object names the
	// chart derives from the release name.
	runID := e2eRunID(t)
	namespace := "gitlabcore-e2e-" + runID
	release := "e2e-" + runID

	releaseLabels := map[string]string{
		render.ReleaseNameLabel:      release,
		render.ReleaseNamespaceLabel: namespace,
	}

	reconciler := e2eReconciler(t, kubeClient)
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: release}}

	e2eCreateNamespace(ctx, t, kubeClient, namespace)
	e2eProvisionManagerAccount(ctx, t, kubeClient, namespace)

	t.Cleanup(func() {
		e2eSweepClusterScoped(t, kubeClient, releaseLabels)
		e2eDeleteNamespace(t, kubeClient, namespace)
	})

	core := e2eGitLabCore(t, release, namespace)
	require.NoError(t, kubeClient.Create(ctx, core))

	t.Run("the first reconcile takes the finalizer, renders, runs the hooks, and applies", func(t *testing.T) {
		// One pass does all of it. A pass that stopped after the finalizer would
		// never be woken up again: adding one changes metadata, not the spec, so
		// the event filter of the controller drops the update.
		//
		// This is also the slow phase: the pre-install hooks block until the
		// shared secrets Job completes, which pulls the toolbox image on a cold
		// cluster.
		result, err := reconciler.Reconcile(ctx, request)
		require.NoError(t, err)

		assert.Contains(t, e2eGetCore(ctx, t, kubeClient, request.NamespacedName).Finalizers,
			finalizerName)

		// The workloads cannot become ready without PostgreSQL, Redis, and
		// object storage, so the loop asks to be woken up instead.
		assert.Equal(t, defaultRequeueDelay, result.RequeueAfter)

		live := e2eGetCore(ctx, t, kubeClient, request.NamespacedName)

		assert.Equal(t, PhasePreparing, live.Status.Phase)
		assert.Equal(t, e2eChartVersion(), live.Status.Version,
			"the version is recorded once the objects are applied")

		initialized := apimeta.FindStatusCondition(live.Status.Conditions, ConditionInitialized)
		require.NotNil(t, initialized)
		assert.Equal(t, metav1.ConditionTrue, initialized.Status)
		assert.Equal(t, reasonChartRendered, initialized.Reason)
		assert.Equal(t, live.Generation, initialized.ObservedGeneration)

		available := apimeta.FindStatusCondition(live.Status.Conditions, ConditionAvailable)
		require.NotNil(t, available)
		assert.Equal(t, metav1.ConditionFalse, available.Status)
		assert.Equal(t, reasonWorkloadsNotReady, available.Reason)
	})

	t.Run("the hooks generated the secrets the chart does not render", func(t *testing.T) {
		secrets := &corev1.SecretList{}
		require.NoError(t, kubeClient.List(ctx, secrets, client.InNamespace(namespace)))

		names := []string{}

		for _, secret := range secrets.Items {
			names = append(names, secret.Name)
		}

		assert.Contains(t, names, release+"-rails-secret")
		assert.Contains(t, names, release+"-gitlab-shell-host-keys")
	})

	t.Run("the applied objects are owned and carry the release labels", func(t *testing.T) {
		webservice := &appsv1.Deployment{}
		key := types.NamespacedName{Namespace: namespace, Name: release + "-webservice-default"}

		require.NoError(t, kubeClient.Get(ctx, key, webservice))

		// The owner reference is what makes the namespaced objects go away with
		// the resource, without the finalizer having to delete them.
		require.Len(t, webservice.OwnerReferences, 1)
		assert.Equal(t, "GitLabCore", webservice.OwnerReferences[0].Kind)
		assert.Equal(t, release, webservice.OwnerReferences[0].Name)
		require.NotNil(t, webservice.OwnerReferences[0].Controller)
		assert.True(t, *webservice.OwnerReferences[0].Controller)

		for key, value := range releaseLabels {
			assert.Equal(t, value, webservice.Labels[key], "label %s", key)
		}

		// The chart labels survive alongside them, and the selector, which is
		// immutable, is untouched.
		assert.Equal(t, "webservice", webservice.Labels["app"])
		assert.NotContains(t, webservice.Spec.Selector.MatchLabels, render.ReleaseNameLabel)
	})

	t.Run("no definition and no RBAC is installed", func(t *testing.T) {
		// The reconciler stamps the release labels on everything it applies, so
		// a definition or a Role of this release would carry them. What an
		// administrator provisioned does not, which is what makes this assertion
		// hold on a cluster that already serves the APIs of the chart and holds
		// RBAC of its own.
		live := e2eGetCore(ctx, t, kubeClient, request.NamespacedName)

		discovered, err := reconciler.capabilities()
		require.NoError(t, err)

		result, err := renderRelease(live, chartsDirectory(), discovered, logr.Discard())
		require.NoError(t, err)

		skipped := objects.Filter(result.Objects, neverApplied)
		t.Logf("the release renders %d definitions and RBAC objects, and carries %d definitions in crds/",
			len(skipped), len(result.CRDs))

		for _, gvk := range e2eSkippedKinds {
			applied := e2eListByLabels(ctx, t, kubeClient, gvk, releaseLabels)
			assert.Empty(t, applied, "the reconciler applies no %s", gvk.Kind)
		}
	})

	t.Run("reconciling again is idempotent and does not rerun the hooks", func(t *testing.T) {
		// A controller reconciles repeatedly. The objects are applied a second
		// time, which has to be a no-op, and the hooks are not rerun: the marker
		// of the first pass says they belong to this generation already.
		before := e2eGetCore(ctx, t, kubeClient, request.NamespacedName)
		require.True(t, hooksAreCurrent(before),
			"the first pass records the generation whose hooks ran")

		result, err := reconciler.Reconcile(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, defaultRequeueDelay, result.RequeueAfter)

		after := e2eGetCore(ctx, t, kubeClient, request.NamespacedName)
		assert.True(t, hooksAreCurrent(after))
	})

	t.Run("deleting the resource sweeps the release and releases the finalizer", func(t *testing.T) {
		live := e2eGetCore(ctx, t, kubeClient, request.NamespacedName)
		require.NoError(t, kubeClient.Delete(ctx, live))

		// The finalizer holds the resource until the reconciler has swept what
		// garbage collection cannot reach.
		live = e2eGetCore(ctx, t, kubeClient, request.NamespacedName)
		assert.NotNil(t, live.DeletionTimestamp)

		result, err := reconciler.Reconcile(ctx, request)
		require.NoError(t, err)
		assert.True(t, result.IsZero())

		err = kubeClient.Get(ctx, request.NamespacedName, &apiv2alpha1.GitLabCore{})
		assert.True(t, apierrors.IsNotFound(err), "the resource is deletable once the finalizer is gone")

		for _, gvk := range e2eClusterScopedKinds {
			leftovers := e2eListByLabels(ctx, t, kubeClient, gvk, releaseLabels)
			assert.Empty(t, leftovers, "%s objects of the release are swept", gvk.Kind)
		}

		// The namespaced objects are collected by Kubernetes, through the owner
		// reference the reconciler set.
		key := types.NamespacedName{Namespace: namespace, Name: release + "-webservice-default"}

		require.Eventually(t, func() bool {
			err := kubeClient.Get(ctx, key, &appsv1.Deployment{})

			return apierrors.IsNotFound(err)
		}, e2eGarbageCollectionTimeout, 2*time.Second,
			"the owned objects are garbage collected with the resource")
	})
}

// e2eRequireChart skips the test when the chart archive under test is not
// staged.
func e2eRequireChart(t *testing.T) {
	t.Helper()

	if os.Getenv("HELM_CHARTS") == "" || e2eChartVersion() == "" {
		t.Skip("HELM_CHARTS and CHART_VERSION are not set; run the test via `task e2e-tests`")
	}

	_, err := render.LocateChart(chartsDirectory(), chartName, e2eChartVersion())
	require.NoError(t, err, "run `task retrieve-charts`")
}

// e2eRequireDefinition skips the test when the cluster does not serve the
// GitLabCore definition, which ships in no release.
func e2eRequireDefinition(ctx context.Context, t *testing.T, kubeClient client.Client) {
	t.Helper()

	err := kubeClient.List(ctx, &apiv2alpha1.GitLabCoreList{}, client.InNamespace("default"))
	if apimeta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
		t.Skip("the cluster does not serve gitlabcores.apps.gitlab.com; run `task install_v2alpha1_crds`")
	}

	require.NoError(t, err)
}

// e2eChartVersion is the chart version under test.
func e2eChartVersion() string {
	return chartVersion()
}

// e2eClient builds a client on the kubeconfig of the environment, with the
// v2alpha1 types registered, the way the manager hands one to the reconciler.
func e2eClient(t *testing.T) client.Client {
	t.Helper()

	runtime.Must(apiv2alpha1.AddToScheme(kubectlscheme.Scheme))

	restConfig, err := config.GetConfig()
	require.NoError(t, err, "no kubeconfig; point KUBECONFIG at a throwaway cluster")

	kubeClient, err := client.New(restConfig, client.Options{Scheme: kubectlscheme.Scheme})
	require.NoError(t, err)

	return kubeClient
}

// e2eReconciler builds the reconciler the manager would register, with the
// discovery client and the Helm client getter it defaults to.
func e2eReconciler(t *testing.T, kubeClient client.Client) *Reconciler {
	t.Helper()

	restConfig, err := config.GetConfig()
	require.NoError(t, err)

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	require.NoError(t, err)

	return &Reconciler{
		Client:           kubeClient,
		Log:              logzap.New(logzap.UseDevMode(true), logzap.WriteTo(e2eWriter{t})),
		Scheme:           kubeClient.Scheme(),
		Recorder:         events.NewFakeRecorder(e2eEventBuffer),
		Discovery:        discoveryClient,
		RESTClientGetter: genericclioptions.NewConfigFlags(true),
		HookTimeout:      e2eHookTimeout,
	}
}

// e2eEventBuffer is large enough that the recorder of the test never blocks on
// an event nobody reads.
const e2eEventBuffer = 128

// e2eGitLabCore builds the resource under test.
func e2eGitLabCore(t *testing.T, release, namespace string) *apiv2alpha1.GitLabCore {
	t.Helper()

	values := support.Values{}
	require.NoError(t, values.AddFromYAML(e2eValues))

	return &apiv2alpha1.GitLabCore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      release,
			Namespace: namespace,
		},
		Spec: apiv2alpha1.GitLabCoreSpec{
			Hostname: testHostname,
			Chart: apiv2alpha1.ChartSpec{
				Version: e2eChartVersion(),
				Values:  apiv2alpha1.ChartValues{Object: values},
			},
		},
	}
}

// e2eGetCore reads the resource under test.
func e2eGetCore(ctx context.Context, t *testing.T, kubeClient client.Client, key types.NamespacedName) *apiv2alpha1.GitLabCore {
	t.Helper()

	core := &apiv2alpha1.GitLabCore{}
	require.NoError(t, kubeClient.Get(ctx, key, core))

	return core
}

// e2eListByLabels lists the objects of one kind that carry the given labels. A
// kind the cluster does not serve lists as nothing, because an object of it
// cannot exist.
func e2eListByLabels(ctx context.Context, t *testing.T, kubeClient client.Client, gvk schema.GroupVersionKind, labels map[string]string) []unstructured.Unstructured {
	t.Helper()

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	err := kubeClient.List(ctx, list, client.MatchingLabels(labels))
	if apimeta.IsNoMatchError(err) {
		return nil
	}

	require.NoError(t, err)

	return list.Items
}

func e2eCreateNamespace(ctx context.Context, t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	require.NoError(t, kubeClient.Create(ctx, namespace))

	t.Logf("created namespace %s", name)
}

// e2eProvisionManagerAccount stands in for the Operator installation.
//
// The shared secrets defaults point the Job at the ServiceAccount of the
// Operator and turn the chart RBAC off, so the account and its permission to
// manage Secrets have to be there already. The Operator chart provisions both;
// in a throwaway namespace the test does.
func e2eProvisionManagerAccount(ctx context.Context, t *testing.T, kubeClient client.Client, namespace string) {
	t.Helper()

	account := settings.ManagerServiceAccount
	meta := metav1.ObjectMeta{Name: account, Namespace: namespace}

	require.NoError(t, kubeClient.Create(ctx, &corev1.ServiceAccount{ObjectMeta: meta}))

	require.NoError(t, kubeClient.Create(ctx, &rbacv1.Role{
		ObjectMeta: meta,
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "list", "create", "patch"},
		}},
	}))

	require.NoError(t, kubeClient.Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: meta,
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     account,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      account,
			Namespace: namespace,
		}},
	}))

	t.Logf("provisioned the %s service account and its Secret permissions", account)
}

func e2eDeleteNamespace(t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	if os.Getenv("E2E_KEEP_NAMESPACE") != "" {
		t.Logf("keeping namespace %s", name)

		return
	}

	// A fresh context: the test context may already be done.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := kubeClient.Delete(ctx, namespace); err != nil && !apierrors.IsNotFound(err) {
		t.Logf("deleting namespace %s: %v", name, err)
	}
}

// e2eSweepClusterScoped deletes the cluster-scoped objects of the release. The
// finalizer of the reconciler does this too; this is the safety net for a run
// that failed before the deletion phase.
func e2eSweepClusterScoped(t *testing.T, kubeClient client.Client, labels map[string]string) {
	t.Helper()

	if os.Getenv("E2E_KEEP_NAMESPACE") != "" {
		t.Logf("keeping the cluster-scoped objects; delete them with -l %s=%s",
			render.ReleaseNameLabel, labels[render.ReleaseNameLabel])

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	for _, gvk := range e2eClusterScopedKinds {
		target := &unstructured.Unstructured{}
		target.SetGroupVersionKind(gvk)

		err := kubeClient.DeleteAllOf(ctx, target, client.MatchingLabels(labels))
		if err != nil && !apierrors.IsNotFound(err) && !apimeta.IsNoMatchError(err) {
			t.Logf("deleting %s by label: %v", gvk.Kind, err)
		}
	}
}

// e2eRunID returns a short lowercase token unique to this run.
func e2eRunID(t *testing.T) string {
	t.Helper()

	buf := make([]byte, 4)
	_, err := rand.Read(buf)
	require.NoError(t, err)

	return hex.EncodeToString(buf)
}

// e2eWriter routes the reconciler log into the test log.
type e2eWriter struct{ t *testing.T }

func (w e2eWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))

	return len(p), nil
}
