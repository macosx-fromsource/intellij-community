package helm

import (
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Query provides access methods to query Helm templates.
type Query interface {
	// Template returns the attached template that this interface queries.
	Template() Template

	// Reset clears the query cache when applicable.
	Reset()

	// ObjectsByKind returns all objects that match the kind specifier. Type specifier can be in the form of
	// Kind, Kind.group, Kind.version.group.
	ObjectsByKind(kindArg string) []client.Object

	// ObjectByKindAndName returns the object that match the kind specifier and has the provided name.
	ObjectByKindAndName(kindArg, name string) client.Object

	// ObjectByKindAndLabels returns the all objects that match the kind specifier and have the labels.
	ObjectsByKindAndLabels(kindArg string, labels map[string]string) []client.Object

	// ObjectByKindAndLabels returns the all objects that match the kind specifier and the "app" label.
	ObjectByKindAndComponent(kindArg, component string) client.Object
}

type cachingQuery struct {
	template Template
	cache    map[string]interface{}
	locker   sync.Locker
}

func newQuery(t Template) Query {
	return &cachingQuery{
		template: t,
		cache:    make(map[string]interface{}),
		locker:   &sync.Mutex{},
	}
}

const (
	anything             = "*"
	appLabel             = "app"
	gitlabComponentLabel = "gitlab.io/component"
)

func (q *cachingQuery) Template() Template {
	return q.template
}

func (q *cachingQuery) Reset() {
	q.clearCache()
}

func (q *cachingQuery) runQuery(key string, query func() interface{}) interface{} {
	result := q.readCache(key)

	if result == nil {
		result = query()
		q.updateCache(key, result)
	}

	return result
}

func (q *cachingQuery) queryObjectsWithKindArg(key, kindArg string, selector ObjectSelector) []client.Object {
	gvk, gk := schema.ParseKindArg(kindArg)
	result := q.runQuery(key,
		func() interface{} {
			objects, err := q.template.GetObjects(
				func(raw runtime.Object) bool {
					obj, ok := raw.(client.Object)
					return ok && matchParsedKindArg(obj, gvk, &gk) && selector(obj)
				},
			)

			if err != nil {
				return nil
			}

			return unsafeConvertObjects(objects)
		},
	)

	return result.([]client.Object)
}

func matchParsedKindArg(object client.Object, qGVK *schema.GroupVersionKind, qGK *schema.GroupKind) bool {
	oGVK := object.GetObjectKind().GroupVersionKind()
	result := false

	if qGVK != nil {
		result = qGVK.Kind == oGVK.Kind &&
			(qGVK.Group == "" || qGVK.Group == oGVK.Group) &&
			(qGVK.Version == "" || qGVK.Version == oGVK.Version)
	}

	if result {
		return true
	}

	return qGK.Kind == oGVK.Kind &&
		(qGK.Group == "" || qGK.Group == oGVK.Group || qGK.Group == oGVK.Version)
}

func matchLabels(oLabels, qLabels map[string]string) bool {
	for k, v := range qLabels {
		if w, ok := oLabels[k]; !ok || v != w {
			return false
		}
	}

	return true
}

func unsafeConvertObjects(objects []runtime.Object) []client.Object {
	result := make([]client.Object, len(objects))
	for i, o := range objects {
		result[i] = o.(client.Object)
	}

	return result
}
