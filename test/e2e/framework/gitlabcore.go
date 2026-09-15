package framework

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2alpha1 "gitlab.com/gitlab-org/cloud-native/gitlab-operator/api/v2alpha1"
)

// CreateBareGitLabCore creates an instance with nothing behind it.
//
// It will never become available: there is no PostgreSQL, no Redis and no object
// storage for it. That is the point. It is how a suite tells the two wait reasons
// of a Siphon apart, because the reference resolves and the instance is still not
// ready.
//
// It is not a substitute for the fixture the siphonrelease suite needs. The
// Operator starts rendering this release, which pulls images, so the case that
// uses it is behind a flag.
func (e *Env) CreateBareGitLabCore(t *testing.T, name string) *apiv2alpha1.GitLabCore {
	t.Helper()

	core := &apiv2alpha1.GitLabCore{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: e.Namespace},
		Spec: apiv2alpha1.GitLabCoreSpec{
			Hostname: "gitlab.e2e.invalid",
			Edition:  apiv2alpha1.EditionEE,
			Chart:    apiv2alpha1.ChartSpec{Version: e.Config.ChartVersion},
		},
	}

	if err := e.Client.Create(e.Ctx, core); err != nil {
		t.Fatalf("creating the GitLabCore %s: %v", name, err)
	}

	return core
}
