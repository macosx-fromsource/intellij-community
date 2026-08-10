package v1beta1

import (
	"github.com/Masterminds/semver/v3"
)

// isZeroDowntimePath determines whether an upgrade path supports zero-downtime upgrades (ZDU).
//
// An upgrade is ZDU-compatible if it meets one of these criteria:
//   - Increments the minor version by exactly one (within the same major version)
//   - Transitions from the final minor version (11) of one major version to the first
//     minor version (0) of the next major version
//
// Returns false for:
//   - Downgrades
//   - Either version parameter being nil
//   - All other upgrade paths
func isZeroDowntimePath(from, to *semver.Version) bool {
	if from == nil || to == nil {
		return false
	}

	if from.GreaterThan(to) {
		return false
	}

	if from.Major() != to.Major() {
		return to.Major() == from.Major()+1 && from.Minor() == 11 && to.Minor() == 0
	}

	if from.Minor() != to.Minor() {
		return to.Minor()-from.Minor() <= 1
	}

	return true
}
