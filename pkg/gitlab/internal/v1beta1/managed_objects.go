package v1beta1

import (
	"context"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/controllers/settings"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube/manifest"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/objects"
)

/* ManagedObjects helpers */

func (w *Adapter) PopulateManagedObjects(objects ...runtime.Object) error {
	for _, o := range objects {
		if obj, ok := o.(client.Object); ok {
			if !w.targetManagedObjects.Contains(obj) {
				w.targetManagedObjects.Append(obj)
			}
		} else {
			return errors.Errorf("could not convert %T to client.Object type", o)
		}
	}

	return nil
}

/* ManagedObjects */

func (w *Adapter) CurrentObjects(ctx context.Context) (objects.Collection, error) {
	initSupportedGVRs()

	return kube.DiscoverManagedObjects(w.source,
		manifest.WithContext(ctx), manifest.WithGroupVersionResources(supportedGVRs...))
}

func (w *Adapter) TargetObjects() objects.Collection {
	return w.targetManagedObjects
}

func initSupportedGVRs() {
	if supportedGVRs != nil {
		return
	}

	supportedGVRs = make([]schema.GroupVersionResource, 0)

	for _, gvr := range potentialSupportedGVRs {
		if settings.IsGroupVersionResourceSupported(gvr.GroupVersion().String(), gvr.Resource) {
			supportedGVRs = append(supportedGVRs, gvr)
		}
	}
}

var potentialSupportedGVRs []schema.GroupVersionResource = []schema.GroupVersionResource{
	{
		Version:  "v1",
		Resource: "configmaps",
	}, {
		Version:  "v1",
		Resource: "services",
	}, {
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}, {
		Group:    "apps",
		Version:  "v1",
		Resource: "statefulsets",
	}, {
		Group:    "apps",
		Version:  "v1",
		Resource: "daemonsets",
	}, {
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}, {
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}, {
		Group:    "batch",
		Version:  "v1",
		Resource: "cronjobs",
	}, {
		Group:    "autoscaling",
		Version:  "v2",
		Resource: "horizontalpodautoscalers",
	}, {
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "servicemonitors",
	}, {
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "podmonitors",
	}, {
		Group:    "monitoring.coreos.com",
		Version:  "v1",
		Resource: "prometheuses",
	}, {
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}, {
		Group:    "gateway.networking.k8s.io",
		Version:  "v1alpha2",
		Resource: "tcproutes",
	}, {
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}, {
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gatewayclasses",
	}, {
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "envoyproxies",
	}, {
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "envoypatchpolicies",
	}, {
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "clienttrafficpolicies",
	}, {
		Group:    "gateway.envoyproxy.io",
		Version:  "v1alpha1",
		Resource: "securitypolicies",
	},
}

var supportedGVRs []schema.GroupVersionResource = nil
