package objects

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The workload mutators reproduce the zero-downtime upgrade operations of the
// v1beta1 controller (controllers/upgrade.go) on rendered, not-yet-applied
// objects. Three deviations from that typed reference: the env upserters
// replace an existing value where addEnvVar only adds when absent, an entry
// using valueFrom is replaced wholesale by a literal value, and removal
// preserves the order of the remaining variables where removeEnvVar reorders
// them.

// deploymentKind is the kind the Deployment-only mutators expect.
const deploymentKind = "Deployment"

// nameField and valueField are the keys of env entries.
const (
	nameField  = "name"
	valueField = "value"
)

// podTemplateKinds are the workload kinds whose pod template lives at
// spec.template. CronJob keeps its template at spec.jobTemplate and is not
// supported, like in the v1beta1 controller.
var podTemplateKinds = []string{deploymentKind, "StatefulSet", "DaemonSet", "ReplicaSet", "Job"}

// SetPaused sets spec.paused on a Deployment. A paused Deployment accepts a
// new pod template without rolling it out, which is how a zero-downtime
// upgrade holds workloads while migrations run.
func SetPaused(obj *unstructured.Unstructured, paused bool) error {
	if err := expectKind(obj, deploymentKind); err != nil {
		return err
	}

	return setNestedField(obj, paused, "spec", "paused")
}

// SetReplicas sets spec.replicas on a Deployment.
func SetReplicas(obj *unstructured.Unstructured, replicas int64) error {
	if err := expectKind(obj, deploymentKind); err != nil {
		return err
	}

	if replicas < 0 {
		return fmt.Errorf("objects: replica count %d of %s %q is negative", replicas, obj.GetKind(), obj.GetName())
	}

	return setNestedField(obj, replicas, "spec", "replicas")
}

// UnsetReplicas removes spec.replicas from a Deployment so that a
// HorizontalPodAutoscaler keeps ownership of the scale.
func UnsetReplicas(obj *unstructured.Unstructured) error {
	if err := expectKind(obj, deploymentKind); err != nil {
		return err
	}

	unstructured.RemoveNestedField(obj.Object, "spec", "replicas")

	return nil
}

// GetReplicas reads spec.replicas from a Deployment. An absent field and an
// explicit count carry different meanings when a HorizontalPodAutoscaler owns
// the scale, so check found.
func GetReplicas(obj *unstructured.Unstructured) (replicas int64, found bool, err error) {
	if err := expectKind(obj, deploymentKind); err != nil {
		return 0, false, err
	}

	replicas, found, err = unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil {
		return 0, false, fmt.Errorf("objects: reading replicas of %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	return replicas, found, nil
}

// SetPodTemplateAnnotation sets an annotation on the pod template of a
// workload, for example a secrets checksum or a restart trigger. It fails
// when the object has no pod template to annotate.
func SetPodTemplateAnnotation(obj *unstructured.Unstructured, key, value string) error {
	if err := expectKind(obj, podTemplateKinds...); err != nil {
		return err
	}

	template, _, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "template")
	if err != nil {
		return fmt.Errorf("objects: reading pod template of %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	if _, ok := template.(map[string]interface{}); !ok {
		return fmt.Errorf("objects: %s %q has no pod template", obj.GetKind(), obj.GetName())
	}

	return setNestedField(obj, value, "spec", "template", "metadata", "annotations", key)
}

// UpsertInitContainerEnv sets an environment variable on the named init
// container of a workload, replacing an existing entry with the same name. It
// fails when the init container is absent.
func UpsertInitContainerEnv(obj *unstructured.Unstructured, containerName, envName, envValue string) error {
	container, err := findContainer(obj, "initContainers", containerName)
	if err != nil {
		return err
	}

	return upsertEnv(obj, container, envName, envValue)
}

// RemoveInitContainerEnv removes every environment variable with the given
// name from the named init container of a workload. It fails when the init
// container is absent.
func RemoveInitContainerEnv(obj *unstructured.Unstructured, containerName, envName string) error {
	container, err := findContainer(obj, "initContainers", containerName)
	if err != nil {
		return err
	}

	return removeEnv(obj, container, envName)
}

// UpsertEnvInAllContainers sets an environment variable on every container of
// the pod template, replacing existing entries with the same name. The
// pre-migrations Job derivation uses it to skip post-deployment migrations.
func UpsertEnvInAllContainers(obj *unstructured.Unstructured, envName, envValue string) error {
	containers, err := containerList(obj, "containers")
	if err != nil {
		return err
	}

	if len(containers) == 0 {
		return fmt.Errorf("objects: %s %q has no containers", obj.GetKind(), obj.GetName())
	}

	for _, item := range containers {
		container, ok := item.(map[string]interface{})
		if !ok {
			return malformedContainerError(obj, "containers")
		}

		if err := upsertEnv(obj, container, envName, envValue); err != nil {
			return err
		}
	}

	return nil
}

// expectKind fails when the object is not one of the expected kinds.
func expectKind(obj *unstructured.Unstructured, kinds ...string) error {
	if slices.Contains(kinds, obj.GetKind()) {
		return nil
	}

	return fmt.Errorf("objects: expected one of %v, got %s %q", kinds, obj.GetKind(), obj.GetName())
}

// setNestedField wraps unstructured.SetNestedField with a contextual error.
func setNestedField(obj *unstructured.Unstructured, value interface{}, fields ...string) error {
	if err := unstructured.SetNestedField(obj.Object, value, fields...); err != nil {
		return fmt.Errorf("objects: mutating %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}

	return nil
}

// containerList returns the named container list of the pod template, for
// example "containers" or "initContainers". A present list with an unexpected
// shape is an error, not a missing list.
func containerList(obj *unstructured.Unstructured, listName string) ([]interface{}, error) {
	if err := expectKind(obj, podTemplateKinds...); err != nil {
		return nil, err
	}

	containers, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "template", "spec", listName)
	if err != nil {
		return nil, fmt.Errorf("objects: reading %s of %s %q: %w", listName, obj.GetKind(), obj.GetName(), err)
	}

	if !found {
		return nil, nil
	}

	list, ok := containers.([]interface{})
	if !ok {
		return nil, fmt.Errorf("objects: %s of %s %q is not a list", listName, obj.GetKind(), obj.GetName())
	}

	return list, nil
}

// findContainer returns the container with the given name from the named
// container list of the pod template.
func findContainer(obj *unstructured.Unstructured, listName, containerName string) (map[string]interface{}, error) {
	containers, err := containerList(obj, listName)
	if err != nil {
		return nil, err
	}

	for _, item := range containers {
		container, ok := item.(map[string]interface{})
		if !ok {
			return nil, malformedContainerError(obj, listName)
		}

		if container[nameField] == containerName {
			return container, nil
		}
	}

	return nil, fmt.Errorf(
		"objects: %s %q has no container %q in %s", obj.GetKind(), obj.GetName(), containerName, listName)
}

// malformedContainerError reports a container entry that is not a map.
func malformedContainerError(obj *unstructured.Unstructured, listName string) error {
	return fmt.Errorf("objects: %s %q has a malformed entry in %s", obj.GetKind(), obj.GetName(), listName)
}

// containerEnvList returns the env list of a container. A present env field
// with an unexpected shape is an error.
func containerEnvList(obj *unstructured.Unstructured, container map[string]interface{}) ([]interface{}, error) {
	value, found := container["env"]
	if !found {
		return nil, nil
	}

	env, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf(
			"objects: env of container %q in %s %q is not a list", container[nameField], obj.GetKind(), obj.GetName())
	}

	return env, nil
}

// upsertEnv sets an environment variable on a container, replacing the first
// entry with the same name and dropping any further duplicates, so the
// resulting value is effective regardless of how Kubernetes resolves
// duplicate names. An entry that uses valueFrom is replaced wholesale by the
// literal value.
func upsertEnv(obj *unstructured.Unstructured, container map[string]interface{}, envName, envValue string) error {
	env, err := containerEnvList(obj, container)
	if err != nil {
		return err
	}

	entry := map[string]interface{}{nameField: envName, valueField: envValue}
	kept := make([]interface{}, 0, len(env)+1)
	replaced := false

	for _, item := range env {
		if variable, ok := item.(map[string]interface{}); ok && variable[nameField] == envName {
			if !replaced {
				kept = append(kept, entry)
				replaced = true
			}

			continue
		}

		kept = append(kept, item)
	}

	if !replaced {
		kept = append(kept, entry)
	}

	container["env"] = kept

	return nil
}

// removeEnv removes every environment variable with the given name from a
// container, leaving the env field untouched when nothing matches and
// dropping it when the removal empties it.
func removeEnv(obj *unstructured.Unstructured, container map[string]interface{}, envName string) error {
	env, err := containerEnvList(obj, container)
	if err != nil {
		return err
	}

	kept := make([]interface{}, 0, len(env))

	for _, item := range env {
		if variable, ok := item.(map[string]interface{}); ok && variable[nameField] == envName {
			continue
		}

		kept = append(kept, item)
	}

	switch {
	case len(kept) == len(env):
		return nil
	case len(kept) == 0:
		delete(container, "env")
	default:
		container["env"] = kept
	}

	return nil
}
