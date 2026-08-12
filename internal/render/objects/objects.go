// Package objects selects and mutates rendered chart objects.
//
// Use it to partition a render into apply phases and to derive
// phase-specific variants, for example a pre-migrations Job. Everything
// operates on in-memory objects before they are applied; nothing here touches
// a cluster.
//
// Selection helpers never fail and preserve the input order. The returned
// slices alias the input objects: deep-copy an object before mutating it when
// the original must stay untouched.
//
// Mutation helpers fail when an object does not have the expected kind or
// shape, so select the right objects with predicates first.
package objects

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The GitLab chart stamps one or both of these labels on the objects of a
// component.
const (
	appLabel             = "app"
	gitlabComponentLabel = "gitlab.io/component"
)

// Predicate reports whether an object matches a selection rule.
type Predicate func(obj *unstructured.Unstructured) bool

// Filter returns the objects that match the predicate.
func Filter(objs []*unstructured.Unstructured, keep Predicate) []*unstructured.Unstructured {
	var matched []*unstructured.Unstructured

	for _, obj := range objs {
		if keep(obj) {
			matched = append(matched, obj)
		}
	}

	return matched
}

// Partition splits the objects into those that match the predicate and those
// that do not.
func Partition(objs []*unstructured.Unstructured, keep Predicate) (matched, rest []*unstructured.Unstructured) {
	for _, obj := range objs {
		if keep(obj) {
			matched = append(matched, obj)
		} else {
			rest = append(rest, obj)
		}
	}

	return matched, rest
}

// First returns the first object that matches the predicate, or nil when none
// does.
func First(objs []*unstructured.Unstructured, keep Predicate) *unstructured.Unstructured {
	for _, obj := range objs {
		if keep(obj) {
			return obj
		}
	}

	return nil
}

// ByKind matches objects of the given kind.
func ByKind(kind string) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		return obj.GetKind() == kind
	}
}

// ByName matches objects with the given name.
func ByName(name string) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		return obj.GetName() == name
	}
}

// ByComponent matches the objects of a chart component, identified by the
// "app" or "gitlab.io/component" label.
func ByComponent(component string) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		labels := obj.GetLabels()

		return labels[appLabel] == component || labels[gitlabComponentLabel] == component
	}
}

// MatchLabels matches objects that carry every given label with the given
// value. An empty desired value matches both an empty and an absent label.
func MatchLabels(labels map[string]string) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		objLabels := obj.GetLabels()

		for key, value := range labels {
			if objLabels[key] != value {
				return false
			}
		}

		return true
	}
}

// And matches objects that match every given predicate.
func And(predicates ...Predicate) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		for _, predicate := range predicates {
			if !predicate(obj) {
				return false
			}
		}

		return true
	}
}

// Or matches objects that match at least one of the given predicates.
func Or(predicates ...Predicate) Predicate {
	return func(obj *unstructured.Unstructured) bool {
		for _, predicate := range predicates {
			if predicate(obj) {
				return true
			}
		}

		return false
	}
}
