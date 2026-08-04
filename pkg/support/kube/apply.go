package kube

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubectl/pkg/util"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyOutcome is the action result of ApplyObject call.
type ApplyOutcome string

const (
	// ObjectUnchanged indicates that the resource has not been changed.
	ObjectUnchanged ApplyOutcome = "unchanged"

	// ObjectCreated indicates that a new resource is created.
	ObjectCreated ApplyOutcome = "created"

	// ObjectUpdated indicates that changes are applied to the existing resource.
	ObjectUpdated ApplyOutcome = "updated"
)

// ApplyConfig is the configuration that is used for applying object changes.
//
// The configuration requires a Kubernetes API Client to work properly. You can
// uses different options to provide the client interface, including through
// runtime Context, via the Manager, or directly from the Reconciler interface.
type ApplyConfig struct {
	Client    client.Client
	Codec     runtime.Codec
	Context   context.Context
	Logger    logr.Logger
	Overwrite bool

	object client.Object
}

// ApplyOption represents an individual option of ApplyObject. The available
// options are:
//
//   - WithClient
//   - WithCodec
//   - WithContext
//   - WithLogger
//   - WithManager
//
// See each option for further details.
type ApplyOption = func(*ApplyConfig)

// ApplyObject creates or patches the given object in the Kubernetes cluster.
//
// It imitates `kubectl apply`: built-in types are patched with a three-way
// strategic merge patch, and everything else, custom resources in particular,
// with a three-way JSON merge patch. It annotates objects with the last
// configuration that was used to create or update them with the same annotation
// that `kubectl apply` uses.
//
// It returns the executed operation and an error.
func ApplyObject(object client.Object, options ...ApplyOption) (ApplyOutcome, error) {
	cfg := defaultApplyConfig(object)
	cfg.applyOptions(options)

	if err := cfg.validateOptions(); err != nil {
		return ObjectUnchanged, err
	}

	return cfg.apply()
}

/* ApplyConfig */

func defaultApplyConfig(object client.Object) *ApplyConfig {
	return &ApplyConfig{
		Codec:   unstructured.UnstructuredJSONScheme,
		Context: context.Background(),
		Logger:  logr.Discard(),

		/* Automatically resolve conflicts between the modified and current
		   configuration by using values from the modified configuration */
		Overwrite: true,

		object: object,
	}
}

func (c *ApplyConfig) applyOptions(options []ApplyOption) {
	for _, option := range options {
		option(c)
	}

	c.Logger = c.Logger.
		WithName("Kube").
		WithValues(
			"name", client.ObjectKeyFromObject(c.object),
			"type", fmt.Sprintf("%T", c.object),
			"kind", c.object.GetObjectKind().GroupVersionKind())
}

func (c *ApplyConfig) validateOptions() error {
	if c.Client == nil {
		return errors.New("missing client interface")
	}

	return nil
}

func (c *ApplyConfig) apply() (ApplyOutcome, error) {
	outcome := ObjectUnchanged

	c.Logger.V(2).Info("applying object")

	/* Serialize the specified object as the modified version. Include the last
	   know configuration annotation in the modified version. */
	modified, err := util.GetModifiedConfiguration(c.object, true, c.Codec)
	if err != nil {
		return ObjectUnchanged, c.wrapObjectError(err, "failed to get modified configuration")
	}

	/* Get the current version of the object from server. */
	c.Logger.V(2).Info("obtaining the current configuration from server")
	err = c.Client.Get(c.Context, client.ObjectKeyFromObject(c.object), c.object)

	switch {
	case kerrors.IsNotFound(err):
		/* Create a new object. */
		err = c.create()
		if err == nil {
			outcome = ObjectCreated
		}
	case err != nil:
		err = c.wrapObjectError(err, "failed to obtain current configuration")
	case err == nil:
		/* Patch the existing object. */
		var patched bool

		patched, err = c.patch(modified)
		if err == nil && patched {
			outcome = ObjectUpdated
		}
	}

	return outcome, err
}

func (c *ApplyConfig) create() error {
	c.Logger.V(2).Info("object does not exist, creating it")

	if err := util.CreateApplyAnnotation(c.object, c.Codec); err != nil {
		return c.wrapObjectError(err, "failed to add apply annotation")
	}

	if err := c.Client.Create(c.Context, c.object); err != nil {
		return c.wrapObjectError(err, "failed to create object")
	}

	c.Logger.V(2).Info("object is created")

	return nil
}

func (c *ApplyConfig) patch(modified []byte) (bool, error) {
	c.Logger.V(2).Info("object exists, patching it")

	p, err := c.calculatePatch(modified)
	if err != nil {
		return false, err
	}

	if c.isEmptyPatch(p) {
		c.Logger.V(2).Info("object is not modified")

		return false, nil
	}

	lastResourceVersion := c.object.GetResourceVersion()

	if err := c.Client.Patch(c.Context, c.object, p); err != nil {
		return false, c.wrapObjectError(err, "failed to patch object")
	}

	c.Logger.V(2).Info("object is patched")

	return c.object.GetResourceVersion() != lastResourceVersion, nil
}

func (c *ApplyConfig) calculatePatch(modified []byte) (client.Patch, error) {
	current, err := runtime.Encode(c.Codec, c.object)
	if err != nil {
		return nil, c.wrapObjectError(err, "failed to serialize current configuration")
	}

	/* Retrieve the original configuration of the object from the annotation. */
	original, err := util.GetOriginalConfiguration(c.object)
	if err != nil {
		return nil, c.wrapObjectError(err, "failed to get original configuration")
	}

	preconditions := []mergepatch.PreconditionFunc{
		/* Ignore read-only metadata attributes. */
		ignoreMetadataKey("creationTimestamp"),
	}

	/* Create the versioned object from the type. */
	versionedObject, err := c.builtinVersionedObject()

	switch {
	case runtime.IsNotRegisteredError(err):
		return c.jsonMergePatch(original, modified, current, preconditions)
	case err != nil:
		return nil, c.wrapObjectError(err, "failed to get instance of versioned object")
	default:
		return c.strategicMergePatch(versionedObject, original, modified, current, preconditions)
	}
}

// jsonMergePatch computes a three way JSON merge patch. It is the only patch
// type the kube-apiserver accepts for CRD-backed resources.
func (c *ApplyConfig) jsonMergePatch(original, modified, current []byte, preconditions []mergepatch.PreconditionFunc) (client.Patch, error) {
	c.Logger.V(2).Info("object kind is not a built-in type, using JSON merge patch")

	preconditions = append(preconditions,
		mergepatch.RequireKeyUnchanged("apiVersion"),
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
	)

	patchData, err := jsonmergepatch.CreateThreeWayJSONMergePatch(
		original, modified, current, preconditions...)
	if err != nil {
		if mergepatch.IsPreconditionFailed(err) {
			return nil, c.wrapObjectError(err, "at least one of apiVersion, kind and name was changed")
		}

		c.logPatchFailure(err, "failed to create merge patch", original, modified, current)

		return nil, c.wrapObjectError(err, "failed to create merge patch")
	}

	return client.RawPatch(types.MergePatchType, patchData), nil
}

// strategicMergePatch computes a three way strategic merge patch from the patch
// metadata of the built-in Go type.
func (c *ApplyConfig) strategicMergePatch(versionedObject runtime.Object, original, modified, current []byte, preconditions []mergepatch.PreconditionFunc) (client.Patch, error) {
	lookupPatchMeta, err := strategicpatch.NewPatchMetaFromStruct(versionedObject)
	if err != nil {
		return nil, c.wrapObjectError(err, "unable to obtain patch meta")
	}

	patchData, err := strategicpatch.CreateThreeWayMergePatch(
		original, modified, current, lookupPatchMeta, c.Overwrite, preconditions...)
	if err != nil {
		c.logPatchFailure(err, "failed to create strategic merge patch", original, modified, current)

		return nil, c.wrapObjectError(err, "failed to create strategic merge patch")
	}

	return client.RawPatch(types.StrategicMergePatchType, patchData), nil
}

func (c *ApplyConfig) logPatchFailure(err error, message string, original, modified, current []byte) {
	c.Logger.V(2).Error(err, message,
		"original", string(original),
		"modified", string(modified),
		"current", string(current),
	)
}

// builtinScheme holds the built-in Kubernetes API types, and only those.
//
// It decides the patch type: the kube-apiserver accepts strategic merge patch
// for built-in types only. CRD-backed resources (Gateway API, Envoy Gateway,
// cert-manager, Prometheus Operator, ...) reject it with 415 Unsupported Media
// Type and have to be patched with a JSON merge patch.
//
// This is deliberately a private scheme and not the caller's: the Operator
// registers its CRD types into the scheme it hands to the manager, so looking a
// kind up there would wrongly report that a custom resource supports strategic
// merge patch.
var builtinScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))

	return s
}()

// builtinVersionedObject returns a new instance of the built-in Go type behind
// the object, or a NotRegisteredError when the object is not a built-in type.
func (c *ApplyConfig) builtinVersionedObject() (runtime.Object, error) {
	return builtinScheme.New(c.object.GetObjectKind().GroupVersionKind())
}

func (c *ApplyConfig) isEmptyPatch(p client.Patch) bool {
	buf, err := p.Data(c.object)
	return err == nil && (string(buf) == "{}" || string(buf) == "{\"metadata\":{}}")
}

func (c *ApplyConfig) wrapObjectError(err error, msg string) error {
	return errors.Wrapf(err, "%s: %s [%T]",
		msg, client.ObjectKeyFromObject(c.object), c.object)
}

/* Private */

func ignoreMetadataKey(keys ...string) mergepatch.PreconditionFunc {
	return func(patch interface{}) bool {
		patchMap, ok := patch.(map[string]interface{})
		if !ok {
			return true
		}

		patchMapMetadata, ok := patchMap["metadata"]
		if !ok {
			return true
		}

		patchMapMetadataMap, ok := patchMapMetadata.(map[string]interface{})
		if !ok {
			return true
		}

		for _, key := range keys {
			delete(patchMapMetadataMap, key)
		}

		return true
	}
}
