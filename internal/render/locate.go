package render

import (
	"fmt"
	"os"
	"path/filepath"
)

// LocateChart returns the path of the packaged chart "<name>-<version>.tgz"
// inside the given directory, typically settings.HelmChartsDirectory. It
// fails with guidance when the archive is not on disk.
func LocateChart(dir, name, version string) (string, error) {
	chartPath := filepath.Join(dir, fmt.Sprintf("%s-%s.tgz", name, version))

	// #nosec G703 -- the directory and chart coordinates come from operator
	// settings and pinned versions, not from external input.
	if _, err := os.Stat(chartPath); err != nil {
		return "", fmt.Errorf("chart %s version %s not found at %s: %w", name, version, chartPath, err)
	}

	return chartPath, nil
}
