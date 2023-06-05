package helm

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	accessor = meta.NewAccessor()
)

func (q *cachingQuery) ObjectsByKind(kindArg string) []client.Object {
	key := q.cacheKey(anything, fmt.Sprintf("%s?", kindArg), nil)
	return q.queryObjectsWithKindArg(key, kindArg, TrueSelector)
}

func (q *cachingQuery) ObjectByKindAndName(kindArg, name string) client.Object {
	key := q.cacheKey(name, fmt.Sprintf("%s?", kindArg), nil)
	objects := q.queryObjectsWithKindArg(key, kindArg, func(obj runtime.Object) bool {
		objName, err := accessor.Name(obj)
		return err == nil && objName == name
	})

	if len(objects) == 0 {
		return nil
	}

	return objects[0]
}

func (q *cachingQuery) ObjectsByKindAndLabels(kindArg string, labels map[string]string) []client.Object {
	key := q.cacheKey(anything, fmt.Sprintf("%s?", kindArg), labels)

	return q.queryObjectsWithKindArg(key, kindArg, func(obj runtime.Object) bool {
		objLabels, err := accessor.Labels(obj)
		return err == nil && matchLabels(objLabels, labels)
	})
}

func (q *cachingQuery) ObjectByKindAndComponent(kindArg, component string) client.Object {
	key := q.cacheKey(anything, fmt.Sprintf("%s?", kindArg), map[string]string{"component": component})

	objects := q.queryObjectsWithKindArg(key, kindArg, func(obj runtime.Object) bool {
		objLabels, err := accessor.Labels(obj)

		if err != nil {
			return false
		}

		return objLabels[appLabel] == component || objLabels[gitlabComponentLabel] == component
	})

	if len(objects) == 0 {
		return nil
	}

	return objects[0]
}
