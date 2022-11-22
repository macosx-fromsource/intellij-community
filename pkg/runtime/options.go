package runtime

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WithLogger uses the specified Logger for the new runtime Context.
//
// This is compatible with logr Context.
func WithLogger(logger logr.Logger) ContextOption {
	return func(cfg *ContextConfig) {
		cfg.Logger = logger
	}
}

// WithClient uses the specified Client for the new runtime Context.
//
// Generally this is the Client that the controller-runtime creates and
// is associated to the Controller.
func WithClient(client client.Client) ContextOption {
	return func(cfg *ContextConfig) {
		cfg.Client = client
	}
}

// WithEventRecorder uses the specified EventRecorder for the new runtime
// Context.
//
// You can obtain an EventRecorder from the controller-runtime Manager.
func WithEventRecorder(recorder record.EventRecorder) ContextOption {
	return func(cfg *ContextConfig) {
		cfg.Recorder = recorder
	}
}
