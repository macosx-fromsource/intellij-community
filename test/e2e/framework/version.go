package framework

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/version"
)

// atLeastVersion reports whether the cluster is at least the given version.
//
// The full git version is preferred because it carries the patch level, and the
// major and the minor are the fallback, with the trailing marker of a decorated
// minor such as "35+" dropped. A managed distribution decorates both.
func atLeastVersion(info *version.Info, minimum string) (bool, error) {
	floor, err := semver.NewVersion(minimum)
	if err != nil {
		return false, fmt.Errorf("the minimum version %q is unreadable: %w", minimum, err)
	}

	parsed, err := parseServerVersion(info)
	if err != nil {
		return false, err
	}

	return !parsed.LessThan(floor), nil
}

func parseServerVersion(info *version.Info) (*semver.Version, error) {
	if parsed, err := semver.NewVersion(strings.TrimPrefix(info.GitVersion, "v")); err == nil {
		return parsed, nil
	}

	major := leadingDigits(info.Major)
	minor := leadingDigits(info.Minor)

	if major == "" || minor == "" {
		return nil, fmt.Errorf("the cluster reports the unreadable version %q", info.GitVersion)
	}

	return semver.NewVersion(major + "." + minor + ".0")
}

// leadingDigits keeps the leading digits of a version component, so that "35+"
// reads as 35.
func leadingDigits(component string) string {
	for index, character := range component {
		if character < '0' || character > '9' {
			return component[:index]
		}
	}

	return component
}
