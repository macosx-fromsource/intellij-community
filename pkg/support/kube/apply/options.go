package apply

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	rt "gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/runtime"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube"
)

/* Apply Options */

// WithClient specifies the Kubernetes API Client that is uses for running
// all cluster operations for ApplyObject.
//
// Note that ApplyObject requires a client and tries different configuration
// points to locate it. However, it does not fall back to a default client.
func WithClient(client client.Client) kube.ApplyOption {
	return func(cfg *kube.ApplyConfig) {
		cfg.Client = client
	}
}

// WithCodec configures apply with the specified codec for serializing and
// deserializing objects.
//
// By default it uses the `UnstructuredJSONScheme` codec.
func WithCodec(codec runtime.Codec) kube.ApplyOption {
	return func(cfg *kube.ApplyConfig) {
		cfg.Codec = codec
	}
}

// WithContext configures apply with the specified context and its associated
// logger and client if it can locate them.
//
// By default ApplyObject uses a Background context.
func WithContext(ctx context.Context) kube.ApplyOption {
	return func(cfg *kube.ApplyConfig) {
		cfg.Context = ctx
		cfg.Logger = logr.FromContextOrDiscard(ctx)
		cfg.Client = rt.ClientFromContext(ctx)
	}
}

// WithLogger configures apply with the specified logger.
//
// By defaults all logs are discarded.
func WithLogger(logger logr.Logger) kube.ApplyOption {
	return func(cfg *kube.ApplyConfig) {
		cfg.Logger = logger
	}
}

// WithManager configures ApplyOption with the client and logger from the
// manager.
//
// This is a convenient way for configuring apply for test environments.
func WithManager(manager manager.Manager) kube.ApplyOption {
	return func(cfg *kube.ApplyConfig) {
		cfg.Client = manager.GetClient()
		cfg.Logger = manager.GetLogger()
	}
}
