package runtime

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ContextConfig is the configuration that is used for building a new runtime
// Context.
type ContextConfig struct {
	Client   client.Client
	Logger   logr.Logger
	Recorder record.EventRecorder
}

// ContextOption represents an individual option of NewContext. The available
// options are:
//
//   - WithClient
//   - WithEventRecorder
//   - WithLogger
//
// See each option for further details.
type ContextOption = func(*ContextConfig)

// NewContext returns a new Builder to build a new runtime Context.
func NewContext(parent context.Context, options ...ContextOption) context.Context {
	ctx := parent

	cfg := &ContextConfig{}
	cfg.applyOptions(options)

	if cfg.Logger != nil {
		ctx = logr.NewContext(ctx, cfg.Logger)
	}

	if cfg.Client != nil {
		ctx = context.WithValue(ctx, clientContextKey{}, cfg.Client)
	}

	if cfg.Recorder != nil {
		ctx = context.WithValue(ctx, recorderContextKey{}, cfg.Recorder)
	}

	return ctx
}

// ClientFromContext returns a Client from the context or nil if client details
// are not found.
func ClientFromContext(ctx context.Context) client.Client {
	if c, ok := ctx.Value(clientContextKey{}).(client.Client); ok {
		return c
	}

	return nil
}

// RecorderFromContext returns a Client from the context or nil if recorder
// details are not found.
func RecorderFromContext(ctx context.Context) record.EventRecorder {
	if r, ok := ctx.Value(clientContextKey{}).(record.EventRecorder); ok {
		return r
	}

	return nil
}

/* Private */

type clientContextKey struct{}

type recorderContextKey struct{}

func (c *ContextConfig) applyOptions(options []ContextOption) {
	for _, option := range options {
		option(c)
	}
}
