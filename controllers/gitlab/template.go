package gitlab

import (
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/helm"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

var (
	store          = newTemplateStore()
	templateLogger = ctrl.Log.WithName("template")
)

// GetTemplate ensures that only one instance of Helm template exists per deployment and
// it is rendered only when needed, e.g. it has changed.
func GetTemplate(adapter gitlab.Adapter) (helm.Template, error) {
	hash := adapter.Hash()

	logger := templateLogger.WithValues(
		"namespace", adapter.Name().Namespace,
		"releaseName", adapter.ReleaseName(),
		"hash", hash)

	if hash != "" {
		template := store.lookup(hash)
		if template != nil {
			logger.V(2).Info("Using the cached template")
			return template, nil
		}
	}

	logger.Info("Rendering a new template.")

	charts, err := adapter.Charts()
	if err != nil {
		return nil, err
	}

	builder, err := helm.NewBuilder(charts)
	if err != nil {
		return nil, err
	}

	builder.SetNamespace(adapter.Name().Namespace)
	builder.SetReleaseName(adapter.ReleaseName())
	builder.EnableHooks()

	template, err := builder.Render(adapter.Values())
	if err != nil {
		logger.Error(err, "Failed to render the template")

		return template, err
	}

	logger.V(1).Info("The template is rendered. Check the warnings (if any).",
		"warnings", len(template.Warnings()))

	if logger.V(1).Enabled() {
		for _, w := range template.Warnings() {
			logger.V(1).Info("Warning: An issue occurred while rendering the Helm template",
				"issue", w)
		}
	}

	logger.V(1).Info("Caching the template.")

	if hash != "" {
		return store.update(adapter.Hash(), template), nil
	} else {
		return template, nil
	}
}

type templateStore struct {
	inventory map[string]helm.Template
	locker    sync.Locker
}

func (s *templateStore) lookup(reference string) helm.Template {
	s.locker.Lock()
	defer s.locker.Unlock()

	return s.inventory[reference]
}

func (s *templateStore) update(reference string, template helm.Template) helm.Template {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.inventory[reference] = template

	return template
}

func newTemplateStore() *templateStore {
	return &templateStore{
		inventory: map[string]helm.Template{},
		locker:    &sync.Mutex{},
	}
}
