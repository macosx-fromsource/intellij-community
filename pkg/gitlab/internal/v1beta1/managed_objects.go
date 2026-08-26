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

const (
	appsGroup                = "apps"
	gatewayNetworkingGroup   = "gateway.networking.k8s.io"
	gatewayEnvoyProxyGroup   = "gateway.envoyproxy.io"
	gatewayEnvoyProxyVersion = "v1alpha1"
)

var potentialSupportedGVRs []schema.GroupVersionResource = []schema.GroupVersionResource{
	{
		Version:  "v1",
		Resource: "configmaps",
	}, {
		Version:  "v1",
		Resource: "services",
	}, {
		Group:    appsGroup,
		Version:  "v1",
		Resource: "deployments",
	}, {
		Group:    appsGroup,
		Version:  "v1",
		Resource: "statefulsets",
	}, {
		Group:    appsGroup,
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
		Group:    gatewayNetworkingGroup,
		Version:  "v1",
		Resource: "httproutes",
	}, {
		Group:    gatewayNetworkingGroup,
		Version:  "v1alpha2",
		Resource: "tcproutes",
	}, {
		Group:    gatewayNetworkingGroup,
		Version:  "v1",
		Resource: "gateways",
	}, {
		Group:    gatewayNetworkingGroup,
		Version:  "v1",
		Resource: "gatewayclasses",
	}, {
		Group:    gatewayNetworkingGroup,
		Version:  "v1",
		Resource: "backendtlspolicies",
	}, {
		Group:    gatewayEnvoyProxyGroup,
		Version:  gatewayEnvoyProxyVersion,
		Resource: "envoyproxies",
	}, {
		Group:    gatewayEnvoyProxyGroup,
		Version:  gatewayEnvoyProxyVersion,
		Resource: "envoypatchpolicies",
	}, {
		Group:    gatewayEnvoyProxyGroup,
		Version:  gatewayEnvoyProxyVersion,
		Resource: "clienttrafficpolicies",
	}, {
		Group:    gatewayEnvoyProxyGroup,
		Version:  gatewayEnvoyProxyVersion,
		Resource: "backendtrafficpolicies",
	}, {
		Group:    gatewayEnvoyProxyGroup,
		Version:  gatewayEnvoyProxyVersion,
		Resource: "securitypolicies",
	},
}

var supportedGVRs []schema.GroupVersionResource = nil
