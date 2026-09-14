package gitlabcore

import (
	"strings"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/internal/render/objects"
)

// targetVersionLabel is the label the GitLab chart stamps on the migrations Job
// with the version of the application the release deploys, the value of its own
// `gitlab.versionTag` helper.
//
// Chart 10.3 added it (gitlab-org/charts/gitlab!5218) for exactly this purpose,
// so it is authoritative where it is present: it accounts for a
// `global.gitlabVersion` override in the free-form values, which the appVersion
// of the chart does not. Charts before 10.3 do not emit it, and neither does a
// release that disables the migrations component, hence the fallback in
// releaseGitLabVersion.
const targetVersionLabel = "gitlab.com/target-version"

// releaseGitLabVersion returns the version of the application the rendered
// release deploys, without the leading `v`, and empty when the render answers
// neither way.
//
// Both sources come from the render itself rather than from the catalog of
// charts the Operator carries on disk, because a release may be rendered from a
// chart pulled at reconcile time (see locateOrPullChart) that the catalog knows
// nothing about.
func releaseGitLabVersion(release *render.Result) string {
	if labeled := objects.First(release.Objects, objects.HasLabel(targetVersionLabel)); labeled != nil {
		return strings.TrimPrefix(labeled.GetLabels()[targetVersionLabel], "v")
	}

	return strings.TrimPrefix(release.AppVersion, "v")
}
