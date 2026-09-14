package framework

import (
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// TestWriter routes a log stream into the log of a test.
//
// It is closed from a cleanup function, and every write after that is dropped:
// t.Log panics once the test has returned, and the harness follows pod logs in
// goroutines that a cancelled context may not have unwound yet. Dropping a late
// line loses nothing that matters; panicking in a background goroutine takes the
// whole test binary down with a stack that points nowhere useful.
type TestWriter struct {
	mu     sync.Mutex
	t      *testing.T
	closed bool
}

// NewTestWriter returns a writer for the log of the test, closed when the test
// ends.
func NewTestWriter(t *testing.T) *TestWriter {
	t.Helper()

	writer := &TestWriter{t: t}
	t.Cleanup(writer.Close)

	return writer
}

// Write implements io.Writer.
func (w *TestWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return len(p), nil
	}

	w.t.Log(strings.TrimRight(string(p), "\n"))

	return len(p), nil
}

// Close stops the writer from touching the test.
func (w *TestWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.closed = true
}

// NewLogger returns a logger that writes into the log of the test, in the
// development format the manager uses, so that a harness line and an Operator
// line read the same way.
func NewLogger(t *testing.T) logr.Logger {
	t.Helper()

	return logzap.New(logzap.UseDevMode(true), logzap.WriteTo(NewTestWriter(t)))
}
