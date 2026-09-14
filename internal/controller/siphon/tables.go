package siphon

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/siphontables"
)

// tablesImageRepository is the OCI repository of the table definitions, built
// from the GitLab repository at the application version an instance deploys.
const tablesImageRepository = "registry.gitlab.com/gitlab-org/gitlab/gitlab-siphon-tables"

// The volume the chart declares for the table definitions, and where the
// definitions have to appear when they come from a ConfigMap.
//
// The chart mounts the image at the root of its filesystem and the init container
// reads db/siphon/tables inside it, matching its --tables-dir flag. A ConfigMap
// has no directories: its keys land flat in the mount, so the mount moves down to
// the directory the files came from.
const (
	tablesVolumeName = "siphon-tables"
	tablesMountPath  = "/etc/tables/" + siphontables.TablesPath
)

// imageVolumeMinVersion is the Kubernetes version from which a cluster may serve
// native OCI image volumes.
//
// Nothing selects a source by it. It is here to name a version in the message
// that a set of definitions too large for a ConfigMap carries, which is the one
// case where mounting the image is the way out.
var imageVolumeMinVersion = semver.MustParse("1.35.0")

// tablesRecheckInterval is how long a resolved digest is trusted before the
// registry is asked again.
//
// The reconcile requeues every 30 seconds and a moving tag changes nothing about
// the generation, so without a floor the loop would ask the registry twice a
// minute forever.
const tablesRecheckInterval = 5 * time.Minute

// resolveTablesImage returns the image reference the table definitions come from.
//
// A reference in the specification wins, for pinning a version the reference does
// not resolve to. Otherwise the tag is built from the application version of the
// referenced instance, in the form the images are published under.
func resolveTablesImage(siphon *apiv2alpha1.Siphon, gitlabVersion string) string {
	if siphon.Spec.Tables.Image != "" {
		return siphon.Spec.Tables.Image
	}

	return fmt.Sprintf("%s:v%s-ee", tablesImageRepository, gitlabVersion)
}

// resolveTablesSource returns the source the definitions come from, never Auto.
//
// Auto resolves to the ConfigMap path, which works everywhere. It used to select
// the image volume by the version of the cluster, but the version is only one of
// two preconditions: the container runtime has to serve image volumes too
// (containerd 2.0 or CRI-O 1.31 and later), and nothing in the Kubernetes API
// reports that. A 1.35 cluster on a runtime without it admits the pod, pulls the
// image, and then fails to create the container with
//
//	invalid mount config for type "bind": field Source must not be empty
//
// which names neither the field nor the runtime. Extracting is a few hundred
// kilobytes of ConfigMap and works on every cluster, so it is the safe default
// until the runtime can be probed. An administrator whose cluster does serve
// image volumes sets spec.tables.source to ImageVolume outright.
func resolveTablesSource(requested apiv2alpha1.TablesSource) apiv2alpha1.TablesSource {
	if requested != apiv2alpha1.TablesSourceAuto && requested != "" {
		return requested
	}

	return apiv2alpha1.TablesSourceConfigMap
}

// tablesConfigMapName is the ConfigMap the extracted definitions are published
// to. It is always in the namespace of the resource, so it is always ownable and
// garbage collection removes it.
func tablesConfigMapName(name string) string {
	return name + "-siphon-tables"
}

// The annotations that record what a published ConfigMap holds, so a reconcile
// can tell whether it is current without reading its data.
const (
	tablesImageAnnotation  = "siphon.apps.gitlab.com/tables-image"
	tablesDigestAnnotation = "siphon.apps.gitlab.com/tables-digest"
)

// reconcileTables puts the table definitions in place for the resolved image and
// records what it did on the status.
//
// On the image volume path there is nothing to do: the chart mounts the image
// itself. On the ConfigMap path the image is pulled, the definitions extracted,
// and a ConfigMap the resource owns is published.
func (r *Reconciler) reconcileTables(ctx context.Context, siphon *apiv2alpha1.Siphon, resolved *Release, log logr.Logger) error {
	siphon.Status.TablesSource = string(resolved.TablesSource)
	siphon.Status.TablesImage = resolved.TablesImage

	if resolved.TablesSource == apiv2alpha1.TablesSourceImageVolume {
		siphon.Status.TablesDigest = ""
		siphon.Status.TablesCheckedAt = nil
		siphon.Status.TablesConfigMap = ""
		siphon.Status.TableCount = 0

		setCondition(siphon, ConditionTablesResolved, metav1.ConditionTrue, reasonTablesMounted,
			fmt.Sprintf("the pods mount the table definitions from %s", resolved.TablesImage))

		return nil
	}

	resolved.TablesConfigMap = tablesConfigMapName(siphon.Name)

	auth, err := r.registryAuth(ctx, siphon, resolved.TablesImage)
	if err != nil {
		return r.tablesFailed(siphon, err)
	}

	options := []siphontables.Option{siphontables.WithAuth(auth)}

	current, err := r.tablesAreCurrent(ctx, siphon, resolved, options, log)
	if err != nil {
		return r.tablesFailed(siphon, err)
	}

	if current {
		siphon.Status.TablesConfigMap = resolved.TablesConfigMap

		setCondition(siphon, ConditionTablesResolved, metav1.ConditionTrue, reasonTablesUnchanged,
			fmt.Sprintf("the %d table definitions of %s are published", siphon.Status.TableCount, siphon.Status.TablesDigest))

		return nil
	}

	fetched, err := siphontables.Fetch(ctx, resolved.TablesImage, options...)
	if err != nil {
		return r.tablesFailed(siphon, err)
	}

	if err := r.publishTables(ctx, siphon, resolved.TablesConfigMap, fetched); err != nil {
		return r.tablesFailed(siphon, err)
	}

	log.Info("published the table definitions",
		"configMap", resolved.TablesConfigMap, "tables", len(fetched.Files),
		"digest", fetched.Digest, "bytes", fetched.TotalBytes)

	now := metav1.Now()

	siphon.Status.TablesDigest = fetched.Digest
	siphon.Status.TablesCheckedAt = &now
	siphon.Status.TablesConfigMap = resolved.TablesConfigMap
	siphon.Status.TableCount = tableCount(len(fetched.Files))

	setCondition(siphon, ConditionTablesResolved, metav1.ConditionTrue, reasonTablesPublished,
		fmt.Sprintf("published %d table definitions from %s", len(fetched.Files), resolved.TablesImage))

	return nil
}

// tablesFailed records a failure to put the definitions in place.
//
// A set that does not fit in a ConfigMap is reported as its own reason: it is a
// configuration problem an administrator acts on rather than a transient failure
// to retry, and the message says what to do about it.
func (r *Reconciler) tablesFailed(siphon *apiv2alpha1.Siphon, err error) error {
	reason := reasonTablesFailed
	message := err.Error()

	var tooLarge *siphontables.TooLargeError
	if errors.As(err, &tooLarge) {
		reason = reasonTablesTooLarge
		message = fmt.Sprintf(
			"%s; upgrade the cluster to Kubernetes %s or later, or set spec.tables.source to ImageVolume",
			message, imageVolumeMinVersion)
	}

	setCondition(siphon, ConditionTablesResolved, metav1.ConditionFalse, reason, message)

	return err
}

// tablesAreCurrent reports whether the published ConfigMap already holds the
// definitions of the resolved image.
//
// The digest, not the tag, is what decides, so a tag that moves is picked up. The
// registry is asked at most once every tablesRecheckInterval, and not at all
// while the recorded digest is fresh and the ConfigMap that carries it is intact.
func (r *Reconciler) tablesAreCurrent(ctx context.Context, siphon *apiv2alpha1.Siphon, resolved *Release, options []siphontables.Option, log logr.Logger) (bool, error) {
	if siphon.Status.TablesDigest == "" || siphon.Status.TablesImage != resolved.TablesImage {
		return false, nil
	}

	published, err := r.publishedTables(ctx, siphon, resolved.TablesConfigMap)
	if err != nil {
		return false, err
	}

	if published == nil ||
		published.Annotations[tablesImageAnnotation] != resolved.TablesImage ||
		published.Annotations[tablesDigestAnnotation] != siphon.Status.TablesDigest {
		return false, nil
	}

	if checked := siphon.Status.TablesCheckedAt; checked != nil && time.Since(checked.Time) < tablesRecheckInterval {
		log.V(2).Info("the table definitions were checked recently, not asking the registry",
			"digest", siphon.Status.TablesDigest, "checked", checked.Time)

		return true, nil
	}

	digest, err := siphontables.Digest(ctx, resolved.TablesImage, options...)
	if err != nil {
		return false, err
	}

	now := metav1.Now()
	siphon.Status.TablesCheckedAt = &now

	return digest == siphon.Status.TablesDigest, nil
}

// publishedTables reads the ConfigMap the definitions were published to, and
// returns nil when it is gone.
func (r *Reconciler) publishedTables(ctx context.Context, siphon *apiv2alpha1.Siphon, name string) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}

	err := r.Get(ctx, types.NamespacedName{Namespace: siphon.Namespace, Name: name}, configMap)

	switch {
	case client.IgnoreNotFound(err) == nil && err != nil:
		return nil, nil
	case err != nil:
		return nil, err
	}

	return configMap, nil
}

// publishTables creates or updates the ConfigMap the resource owns.
//
// The release labels go on alongside the owner reference: garbage collection is
// what removes it, and the labels are what finds it again if the reference is ever
// lost.
func (r *Reconciler) publishTables(ctx context.Context, siphon *apiv2alpha1.Siphon, name string, fetched *siphontables.Result) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: siphon.Namespace},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, configMap, func() error {
		configMap.Data = fetched.Files

		if configMap.Annotations == nil {
			configMap.Annotations = map[string]string{}
		}

		// What the ConfigMap holds, so the next pass can tell whether it is
		// current without reading its data.
		configMap.Annotations[tablesImageAnnotation] = siphon.Status.TablesImage
		configMap.Annotations[tablesDigestAnnotation] = fetched.Digest

		if configMap.Labels == nil {
			configMap.Labels = map[string]string{}
		}

		configMap.Labels[render.ReleaseNameLabel] = siphon.Name
		configMap.Labels[render.ReleaseNamespaceLabel] = siphon.Namespace

		return controllerutil.SetControllerReference(siphon, configMap, r.Scheme)
	})

	return err
}

// tableCount narrows a count for the status field, which is an int32 like every
// count in a Kubernetes API. The set cannot approach the limit: it is capped at
// 900 KiB and the smallest useful definition is tens of bytes, so this saturates
// rather than wraps only to keep the conversion honest.
func tableCount(files int) int32 {
	if files > math.MaxInt32 {
		return math.MaxInt32
	}

	return int32(files) //nolint:gosec // Bounded by the check above.
}

// registryAuth builds the authenticator the definitions are pulled with.
//
// Without a pull secret the pull is anonymous, which is what the published image
// needs. The ambient keychain is never used: it reads the Docker configuration of
// the current user, which does not exist in the Operator pod and would make the
// behavior differ between a developer machine and a cluster.
func (r *Reconciler) registryAuth(ctx context.Context, siphon *apiv2alpha1.Siphon, image string) (authn.Authenticator, error) {
	if siphon.Spec.Tables.PullSecretRef == nil {
		return authn.Anonymous, nil
	}

	secret := &corev1.Secret{}

	key := types.NamespacedName{Namespace: siphon.Namespace, Name: siphon.Spec.Tables.PullSecretRef.Name}
	if err := r.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("reading the pull secret %q: %w", key.Name, err)
	}

	config, found := secret.Data[corev1.DockerConfigJsonKey]
	if !found {
		return nil, fmt.Errorf(
			"the pull secret %q has no %s key, so it is not a kubernetes.io/dockerconfigjson Secret",
			key.Name, corev1.DockerConfigJsonKey)
	}

	return siphontables.AuthFromDockerConfig(config, image)
}

// mountTables rewrites the release so the pods read the definitions from the
// published ConfigMap instead of an image volume.
//
// It is a rewrite rather than a value because the chart emits the image volume
// unconditionally in split mode and appends the volumes a value supplies rather
// than replacing them, so no value can take the image volume away.
func mountTables(objs []*unstructured.Unstructured, configMapName string) error {
	for _, obj := range objects.Filter(objs, objects.ByKind("Deployment")) {
		if err := objects.ReplaceImageVolumeWithConfigMap(
			obj, tablesVolumeName, configMapName, tablesMountPath); err != nil {
			return err
		}
	}

	return nil
}
