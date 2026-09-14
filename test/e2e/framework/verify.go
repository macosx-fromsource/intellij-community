package framework

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// Log fragments the verification looks for.
//
// The absence of a line is the only signal that the image was built without the
// bridge tag: cmd/manager/siphon_stub.go returns without logging anything. So a
// verification that does not look for these passes happily against an Operator
// that does nothing at all, which is the failure this whole file exists to
// prevent.
const (
	siphonDisabledLog = "Siphon reconciler disabled"
	execFormatError   = "exec format error"

	// managerStartedLog is what controller-runtime logs once a controller is up.
	// It is the marker that says the manager has finished registering, so the
	// absence of a Siphon line from here on is an answer rather than impatience.
	managerStartedLog = "Starting workers"
)

// siphonControllerLog matches the line controller-runtime logs when it starts the
// Siphon controller, which Named("siphon") produces.
//
// It is a pattern rather than a literal because zap encodes the field differently
// per mode: production JSON writes `"controller":"siphon"` and the development
// console writes `"controller": "siphon"`, and manager.debug.enabled decides
// which. Matching one literal would make the guard pass or fail on a log format
// rather than on whether the controller is running.
var siphonControllerLog = regexp.MustCompile(`"controller"\s*:\s*"siphon"`)

// VerifySiphonController proves the Siphon reconciler is running, in three layers.
func (h *Harness) verifySiphonController(ctx context.Context) error {
	if err := h.verifyBridgeEnv(ctx); err != nil {
		return err
	}

	if err := h.verifyManagerLog(ctx); err != nil {
		return err
	}

	return h.verifyReconcileLoop(ctx)
}

// verifyBridgeEnv is the static layer: the chart value reached the pod.
func (h *Harness) verifyBridgeEnv(ctx context.Context) error {
	deployment, err := h.managerDeployment(ctx)
	if err != nil {
		return fmt.Errorf("reading the Deployment of the Operator: %w", err)
	}

	container, err := managerContainerOf(deployment)
	if err != nil {
		return err
	}

	if !hasEnv(container, enableBridgeVar, trueValue) {
		return fmt.Errorf(
			"the manager container carries no %s=true, so the Operator runs no Siphon reconciler; "+
				"install it with `--set bridge.enabled=true`", enableBridgeVar)
	}

	return nil
}

// verifyManagerLog is the log layer: the binary was built with the bridge tag.
//
// The manager Deployment is already Available by the time this runs, so the wait
// is only for the log of a started controller to appear. Once it has, the Siphon
// line either is there or never will be, and waiting out a long budget would only
// delay a clear answer.
func (h *Harness) verifyManagerLog(ctx context.Context) error {
	var logs string

	err := pollUntil(ctx, ShortTimeout, func(ctx context.Context) (bool, error) {
		collected, err := h.managerLogs(ctx)

		// A platform mismatch is terminal: the container will never run, so polling
		// on would only replace a precise message with a vague one.
		var mismatch *platformMismatchError
		if errors.As(err, &mismatch) {
			return false, err
		}

		if err != nil {
			// The container may not be serving its log yet.
			return false, nil //nolint:nilerr // a log that is not there yet is not a failure
		}

		logs = collected

		return strings.Contains(logs, managerStartedLog) ||
			strings.Contains(logs, siphonDisabledLog), nil
	})

	var mismatch *platformMismatchError
	if errors.As(err, &mismatch) {
		return err
	}

	switch {
	case siphonControllerLog.MatchString(logs):
		h.Log.Info("the Operator has the Siphon controller running")

		return nil

	case strings.Contains(logs, siphonDisabledLog):
		return fmt.Errorf(
			"the Operator logs %q, so the image carries the bridge tag but the runtime flag is off; "+
				"install it with `--set bridge.enabled=true`", siphonDisabledLog)

	case strings.Contains(logs, managerStartedLog):
		return errors.New(
			"the Operator started its controllers and none of them is the Siphon one, and it does not " +
				"report the reconciler as disabled either, so the image was built from Dockerfile rather " +
				"than Dockerfile.bridge; build it with `task docker-build-bridge`")

	case err != nil:
		return fmt.Errorf("the Operator log never showed a started controller: %w", err)

	default:
		return errors.New("the Operator log never showed a started controller")
	}
}

// managerLogs is the log of every manager pod, joined, plus the platform check:
// an image for the wrong architecture crash-loops, and the log of a container that
// never ran is empty, which would otherwise read as a missing build tag.
func (h *Harness) managerLogs(ctx context.Context) (string, error) {
	pods, err := h.PodsForLabels(ctx, h.Config.OperatorNamespace,
		map[string]string{"control-plane": "controller-manager"})
	if err != nil {
		return "", err
	}

	var combined strings.Builder

	for _, pod := range pods {
		if err := h.checkPodPlatform(pod); err != nil {
			return "", err
		}

		logs, err := h.PodLog(ctx, pod.Namespace, pod.Name, managerContainer, 0)
		if err != nil {
			return "", err
		}

		combined.WriteString(logs)
	}

	return combined.String(), nil
}

// checkPodPlatform turns an architecture mismatch into the error it really is.
//
// An image built for another architecture crash-loops with an exec format error,
// which reads as a broken Operator rather than a broken build. It is the failure
// mode of loading an amd64 image into an arm64 k3s node, and the other way round.
func (h *Harness) checkPodPlatform(pod corev1.Pod) error {
	for _, status := range pod.Status.ContainerStatuses {
		message := ""

		switch {
		case status.State.Terminated != nil:
			message = status.State.Terminated.Message
		case status.LastTerminationState.Terminated != nil:
			message = status.LastTerminationState.Terminated.Message
		case status.State.Waiting != nil:
			message = status.State.Waiting.Message
		}

		if strings.Contains(strings.ToLower(message), execFormatError) {
			return &platformMismatchError{message: message}
		}
	}

	return nil
}

// platformMismatchError reports an image built for another architecture than the
// nodes. It is a type of its own so that the log wait can stop at once rather than
// poll out its budget on a container that will never run.
type platformMismatchError struct {
	message string
}

func (e *platformMismatchError) Error() string {
	return fmt.Sprintf(
		"the manager container reports %q, so the image was built for another architecture than the "+
			"cluster nodes; set E2E_K3S_PLATFORM, or rebuild without a --platform override: %s",
		execFormatError, e.message)
}

// verifyReconcileLoop is the behavioural layer, and the one worth gating on.
//
// It needs no GitLab: the reconciler records the topology before it resolves the
// reference, so a Siphon that will never render still gets its publication name
// written on the first pass. That makes this one poll prove that the definition is
// served, that the controller is watching, and that the Operator may write both
// the finalizer and the status subresource, which is the RBAC check for free.
func (h *Harness) verifyReconcileLoop(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "e2e-operator-probe-" + h.probeSuffix()},
	}

	if err := h.Client.Create(ctx, namespace); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating the probe namespace: %w", err)
	}

	defer func() {
		if err := h.Client.Delete(context.WithoutCancel(ctx), namespace); err != nil {
			h.Log.V(1).Info("could not delete the probe namespace", "error", err)
		}
	}()

	siphon := ProbeSiphon(namespace.Name)
	if err := h.Client.Create(ctx, siphon); err != nil {
		return fmt.Errorf("creating the probe Siphon: %w", err)
	}

	var lastPhase string

	err := pollUntil(ctx, operatorProbeBudget, func(ctx context.Context) (bool, error) {
		live := &apiv2alpha1.Siphon{}
		if err := h.Client.Get(ctx, client.ObjectKeyFromObject(siphon), live); err != nil {
			return false, err
		}

		lastPhase = live.Status.Phase

		return live.Status.Publication == ExpectedPublication, nil
	})

	// Deleted with the namespace, but the finalizer would keep the namespace
	// terminating, so the resource goes first.
	if deleteErr := h.Client.Delete(context.WithoutCancel(ctx), siphon); deleteErr != nil {
		h.Log.V(1).Info("could not delete the probe Siphon", "error", deleteErr)
	}

	if err != nil {
		// The poll error is only ever a deadline, so it is dropped: the budget and
		// the phase say everything, and appending a rate-limiter message to an
		// otherwise clear diagnosis just buries it.
		return fmt.Errorf(
			"the Operator did not record the topology of a Siphon within %s (its phase was %q), so it is "+
				"not reconciling: check that `task install_v2alpha1_crds` granted the manager "+
				"siphons and siphons/status", operatorProbeBudget, lastPhase)
	}

	return nil
}

func (h *Harness) probeSuffix() string {
	id, err := RunID()
	if err != nil {
		return "fallback"
	}

	return id
}
