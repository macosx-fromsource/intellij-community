// Package capabilities reads the cluster facts a chart renders against.
//
// Pass the result to every render. Charts branch on
// .Capabilities.APIVersions.Has, so without policy/v1/PodDisruptionBudget in
// the set the GitLab chart renders PodDisruptionBudget as policy/v1beta1,
// which no current cluster serves, and every apply of that object fails.
package capabilities

import (
	"fmt"
	"path"

	"helm.sh/helm/v4/pkg/chart/common"

	"k8s.io/client-go/discovery"
)

// Capabilities are the cluster facts a chart renders against. The fields map
// onto render.Request, so a discovered set can be passed straight through.
type Capabilities struct {
	// KubeVersion is the version of the API server.
	KubeVersion *common.KubeVersion

	// APIVersions are the group versions and group-version-kinds the
	// cluster serves, in the form .Capabilities.APIVersions.Has expects.
	// The set is complete, as render.Request requires: a non-empty set
	// there replaces the Helm defaults instead of extending them.
	APIVersions []string
}

// Discover reads the capabilities of a cluster.
//
// A cached discovery client is invalidated first, so that definitions
// installed earlier in the same reconcile are visible. Empty discovery is an
// error rather than a set, because the caller renders against the result as a
// complete reading of the cluster.
func Discover(client discovery.DiscoveryInterface) (*Capabilities, error) {
	if cached, ok := client.(discovery.CachedDiscoveryInterface); ok {
		cached.Invalidate()
	}

	serverVersion, err := client.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("capabilities: reading the server version: %w", err)
	}

	apiVersions, err := buildVersionSet(client)
	if err != nil {
		return nil, err
	}

	return &Capabilities{
		KubeVersion: &common.KubeVersion{
			Version: serverVersion.GitVersion,
			Major:   serverVersion.Major,
			Minor:   serverVersion.Minor,
		},
		APIVersions: apiVersions,
	}, nil
}

// buildVersionSet enumerates what the cluster serves, in both the
// group-version form ("policy/v1") and the group-version-kind form
// ("policy/v1/PodDisruptionBudget"), because charts probe both.
//
// This is the walk action.GetVersionSet performs, reproduced here to keep
// pkg/action out of the dependency graph. It differs in the empty case: the
// SDK answers empty discovery with the Helm default set, which describes the
// client-go scheme rather than a cluster and claims group versions such as
// extensions/v1beta1. An API server never discovers empty, so fail instead.
func buildVersionSet(client discovery.ServerResourcesInterface) ([]string, error) {
	// Partial discovery is normal when an aggregated API server is
	// unavailable, and the groups that did resolve are still usable. Any
	// other error means the cluster could not be read at all.
	groups, resources, err := client.ServerGroupsAndResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, fmt.Errorf("capabilities: reading the served api versions: %w", err)
	}

	if len(groups) == 0 && len(resources) == 0 {
		return nil, fmt.Errorf("capabilities: the cluster reported no api versions")
	}

	seen := map[string]bool{}
	versions := []string{}

	add := func(entry string) {
		if !seen[entry] {
			seen[entry] = true
			versions = append(versions, entry)
		}
	}

	for _, group := range groups {
		for _, version := range group.Versions {
			add(version.GroupVersion)
		}
	}

	for _, list := range resources {
		for _, resource := range list.APIResources {
			// A kind can be served at one group version by more than one
			// resource, so the entry is deduplicated rather than appended.
			add(path.Join(list.GroupVersion, resource.Kind))
		}
	}

	return versions, nil
}
