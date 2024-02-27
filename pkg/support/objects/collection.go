package objects

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Collection is a list of objects.
type Collection []client.Object

// Empty returns true if the collection is empty.
func (c Collection) Empty() bool {
	return len(c) == 0
}

// First returns the first item in the collection or `nil` if the collection is
// empty.
func (c Collection) First() client.Object {
	if len(c) > 0 {
		return c[0]
	}

	return nil
}

// Contains checks if the specified object exists.
//
// It compares name, namespaces, and kind (excluding its group and version) of
// the object.
func (c Collection) Contains(object client.Object) bool {
	for _, i := range c {
		if isMatchingObject(i, object) {
			return true
		}
	}

	return false
}

// Filter returns a new collection of objects that satisfy the predicate.
func (c Collection) Filter(predicate Selector) Collection {
	result := Collection{}

	for _, o := range c {
		if predicate(o) {
			result.Append(o)
		}
	}

	return result
}

// Query filters the collection with the specified selectors and returns a new
// collection of matching objects.
//
// When multiple selectors are specified an object must match _all_ of them for
// being selected.
func (c Collection) Query(selectors ...Selector) Collection {
	if len(selectors) == 0 {
		return Collection{}
	}

	return c.Filter(All(selectors...))
}

// Intersection returns a new collection with the objects that are in the
// other collection as well.
func (c Collection) Intersection(other Collection) Collection {
	return c.Filter(other.Contains)
}

// Difference returns a new collection with the objects that are not in the
// other collection.
func (c Collection) Difference(other Collection) Collection {
	return c.Filter(Negate(other.Contains))
}

// Edit applies changes in-place to all objects of the collection using the
// specified editors.
//
// When multiple editors are passed they are applied to each object in the
// specified order.
//
// It returns the number of objects that are changed. When an error occurs
// in one of the editors it stops the change and returns the error immediately
// unless it is a type mismatch error which is used by the Editor to signal that
// it can not edit the object due to type restrictions.
func (c Collection) Edit(editors ...Editor) (int, error) {
	count := 0

	for i := 0; i < len(c); i++ {
		o := c[i]
		for _, editor := range editors {
			err := editor(o)
			if err != nil {
				if IsTypeMismatchError(err) {
					continue
				}

				return count, err
			}
		}

		count++
	}

	return count, nil
}

// Clone creates a copy of the collection and its objects. It uses DeepCopy
// method of Kubernetes objects to duplicate them.
func (c Collection) Clone() Collection {
	result := make(Collection, len(c))

	for i := 0; i < len(c); i++ {
		/*
		 *  CAUTION: We are using unsafe type assertion
		 */
		result[i] = c[i].DeepCopyObject().(client.Object)
	}

	return result
}

// Append inserts the specified objects with the same order at the end of the
// collection.
func (c *Collection) Append(objects ...client.Object) {
	*c = append(*c, objects...)
}

/* Private */

func isMatchingObject(a, b client.Object) bool {
	return a.GetName() == b.GetName() && a.GetNamespace() == b.GetNamespace() &&
		a.GetObjectKind().GroupVersionKind().Kind == b.GetObjectKind().GroupVersionKind().Kind
}
