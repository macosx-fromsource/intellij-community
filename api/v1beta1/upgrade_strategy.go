package v1beta1

const (
	DisableZDUAnnotationKey = "gitlab.io/disable-zero-downtime-upgrade"
)

// IsZeroDowntimeUpgradeDisabled returns true if the GitLab instance has
// the annotation to disable zero-downtime upgrades set to "true".
func IsZeroDowntimeUpgradeDisabled(gitlab *GitLab) bool {
	if gitlab == nil {
		return false
	}

	annotations := gitlab.GetAnnotations()
	if annotations == nil {
		return false
	}

	return annotations[DisableZDUAnnotationKey] == "true"
}
