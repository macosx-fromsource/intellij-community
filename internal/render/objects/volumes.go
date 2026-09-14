package objects

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The volume mutators rewrite the pod template of a rendered, not-yet-applied
// workload. They exist for the case a chart expresses only one way of supplying
// a directory to a pod and the Operator has to supply it another way, which no
// chart value can express.

// The pod template fields the mutators walk.
const (
	volumesField      = "volumes"
	volumeMountsField = "volumeMounts"
	imageField        = "image"
	configMapField    = "configMap"
	mountPathField    = "mountPath"
)

// ReplaceImageVolumeWithConfigMap swaps a native OCI image volume for a
// ConfigMap volume and repoints every mount of it, in the containers and the
// init containers alike, at mountPath.
//
// An image volume presents the whole filesystem of an image, so a chart mounts
// it at the root of the tree it needs and the consumer reads a path inside. A
// ConfigMap has no directories: its keys land flat in the mount, so the mount
// has to move down to the directory the files came from. That is why the new
// mount path is a parameter rather than the one the chart wrote.
//
// A workload without a volume of that name is left untouched, so the mutator
// can run over a whole release. Mounting a ConfigMap this way replaces whatever
// else the chart put at that path, which is the point: the image volume it
// stands in for did the same.
func ReplaceImageVolumeWithConfigMap(obj *unstructured.Unstructured, volumeName, configMapName, mountPath string) error {
	volumes, err := volumeList(obj)
	if err != nil {
		return err
	}

	replaced := false

	for _, item := range volumes {
		volume, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("objects: %s %q has a malformed entry in %s", obj.GetKind(), obj.GetName(), volumesField)
		}

		if volume[nameField] != volumeName {
			continue
		}

		// The source is replaced rather than merged: a volume carries exactly
		// one, and leaving the image source in place makes the object invalid.
		delete(volume, imageField)
		volume[configMapField] = map[string]interface{}{nameField: configMapName}

		replaced = true
	}

	if !replaced {
		return nil
	}

	if err := setNestedField(obj, volumes, "spec", "template", "spec", volumesField); err != nil {
		return err
	}

	return RepointVolumeMount(obj, volumeName, mountPath)
}

// RepointVolumeMount sets the mount path of every mount of the named volume, in
// the containers and the init containers alike. A workload that does not mount
// the volume is left untouched.
func RepointVolumeMount(obj *unstructured.Unstructured, volumeName, mountPath string) error {
	for _, listName := range []string{containersField, initContainersField} {
		containers, err := containerList(obj, listName)
		if err != nil {
			return err
		}

		for _, item := range containers {
			container, ok := item.(map[string]interface{})
			if !ok {
				return malformedContainerError(obj, listName)
			}

			if err := repointMounts(obj, container, listName, volumeName, mountPath); err != nil {
				return err
			}
		}

		if containers == nil {
			continue
		}

		if err := setNestedField(obj, containers, "spec", "template", "spec", listName); err != nil {
			return err
		}
	}

	return nil
}

// repointMounts rewrites the mount path of the named volume in one container.
func repointMounts(obj *unstructured.Unstructured, container map[string]interface{}, listName, volumeName, mountPath string) error {
	value, found := container[volumeMountsField]
	if !found {
		return nil
	}

	mounts, ok := value.([]interface{})
	if !ok {
		return fmt.Errorf("objects: %s of a container in %s of %s %q is not a list",
			volumeMountsField, listName, obj.GetKind(), obj.GetName())
	}

	for _, item := range mounts {
		mount, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("objects: %s of a container in %s of %s %q has a malformed entry",
				volumeMountsField, listName, obj.GetKind(), obj.GetName())
		}

		if mount[nameField] == volumeName {
			mount[mountPathField] = mountPath
		}
	}

	return nil
}

// volumeList returns the volume list of the pod template. A present volumes
// field with an unexpected shape is an error, not a missing list.
func volumeList(obj *unstructured.Unstructured) ([]interface{}, error) {
	if err := expectKind(obj, podTemplateKinds...); err != nil {
		return nil, err
	}

	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "template", "spec", volumesField)
	if err != nil {
		return nil, fmt.Errorf("objects: reading %s of %s %q: %w", volumesField, obj.GetKind(), obj.GetName(), err)
	}

	if !found {
		return nil, nil
	}

	volumes, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("objects: %s of %s %q is not a list", volumesField, obj.GetKind(), obj.GetName())
	}

	return volumes, nil
}
