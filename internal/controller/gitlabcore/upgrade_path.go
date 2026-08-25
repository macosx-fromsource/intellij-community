package gitlabcore

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/charts"
)

// GitLab publishes eleven minor releases before a major bump, so 11 is the last
// minor of a major and the point the chart version rolls over with it. The
// GitLab chart tracks the GitLab version minor-for-minor (chart 10.2 is GitLab
// 19.2), so the whole zero-downtime path can be reasoned about on chart versions
// alone, without mapping to the appVersion.
const lastMinorBeforeMajor = 11

// zeroDowntimePath reports whether an upgrade from one chart version to another
// can be carried out without downtime.
//
// It holds for a single minor step within a major, and for the roll from the
// last minor of a major to the first minor of the next. It is the v1beta1 rule
// (api/v1beta1/upgrade_helper.go), reproduced here because that package is
// frozen and the function is unexported.
func zeroDowntimePath(from, to *semver.Version) bool {
	if from == nil || to == nil {
		return false
	}

	if from.GreaterThan(to) {
		return false
	}

	if from.Major() != to.Major() {
		return to.Major() == from.Major()+1 && from.Minor() == lastMinorBeforeMajor && to.Minor() == 0
	}

	if from.Minor() != to.Minor() {
		return to.Minor()-from.Minor() <= 1
	}

	return true
}

// nextMinor returns the major and minor of the release one minor step above the
// given version, rolling the major over past the last minor.
func nextMinor(version *semver.Version) (major, minor uint64) {
	if version.Minor() == lastMinorBeforeMajor {
		return version.Major() + 1, 0
	}

	return version.Major(), version.Minor() + 1
}

// nextChartVersion returns the chart version an upgrade from deployed toward
// target converges to next: target itself when the step is already a single
// zero-downtime hop, or otherwise the highest patch of the next minor that the
// Operator carries.
//
// It is what splits a multi-minor upgrade into the sequence of single-minor
// upgrades zero-downtime requires. Each call advances one minor; the reconcile
// records the intermediate version and calls it again until it reaches target.
//
// A required intermediate minor the Operator does not carry is a loud error, not
// a skipped minor: skipping one would run migrations across a gap GitLab does not
// support. The caller surfaces it and stops, leaving recovery to an operator who
// adds the chart or changes the spec.
func nextChartVersion(cat charts.Catalog, deployed, target string) (string, error) {
	from, err := semver.NewVersion(deployed)
	if err != nil {
		return "", fmt.Errorf("parsing the deployed chart version %q: %w", deployed, err)
	}

	to, err := semver.NewVersion(target)
	if err != nil {
		return "", fmt.Errorf("parsing the target chart version %q: %w", target, err)
	}

	// A single hop, whether a minor step or a patch within the same minor, is one
	// upgrade cycle and needs no stepping.
	if zeroDowntimePath(from, to) {
		return target, nil
	}

	major, minor := nextMinor(from)

	step, err := highestPatch(cat, major, minor, to)
	if err != nil {
		return "", err
	}

	return step, nil
}

// highestPatch returns the highest chart version the Operator carries whose
// major and minor match and that does not exceed the target, or an error naming
// the missing minor when it carries none.
func highestPatch(cat charts.Catalog, major, minor uint64, target *semver.Version) (string, error) {
	// Versions is sorted descending, so the first match is the highest patch.
	for _, candidate := range cat.Versions(chartName) {
		version, err := semver.NewVersion(candidate)
		if err != nil {
			continue
		}

		if version.Major() == major && version.Minor() == minor && !version.GreaterThan(target) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf(
		"the intermediate chart version %d.%d is required to upgrade to %s one minor at a time, "+
			"but the Operator carries none matching it; it carries %s",
		major, minor, target, availableChartVersions())
}
