package charts

import (
	"cmp"
	"log"
	"slices"

	"github.com/mitchellh/copystructure"
	"golang.org/x/mod/semver"
	chart "helm.sh/helm/v4/pkg/chart/v2"
)

// Catalog is a list of available Helm Charts. Use different criteria with Query
// functions to select the desired Charts from the catalog.
type Catalog []*chart.Chart

// Query selects any chart that matches all of the specified criteria. It can
// return an empty list when it can not find any match.
//
// Each chart must match all the criteria. Use alternative criteria builders for
// different matching requirements.
func (c Catalog) Query(criteria ...Criterion) Catalog {
	result := Catalog{}

	if len(criteria) == 0 {
		return result
	}

	for _, chart := range c {
		if All(criteria...)(chart) {
			/*
			 * NB: We ignore the error because we can not handle it here. When
			 *     this error occurs it means that a fundamental assumption
			 *     about Chart data structure is wrong.
			 */
			cc, err := clone(chart)
			if err != nil {
				log.Printf("WARNING: Chart catalog is unable to clone %s.", chart.Name())
			}

			result = append(result, cc)
		}
	}

	return result
}

// Empty returns true when the catalog is empty. This is useful to check the
// results from the Query function.
func (c Catalog) Empty() bool {
	return len(c) == 0
}

// First returns the first element of the catalog or nil if the catalog is
// empty. This is useful to retrieve results from the Query function.
func (c Catalog) First() *chart.Chart {
	if len(c) > 0 {
		return c[0]
	}

	return nil
}

// Names returns the list of the names of the Charts in this catalog.
func (c Catalog) Names() []string {
	return c.collect(func(chart *chart.Chart) string {
		return chart.Metadata.Name
	})
}

// Versions returns the list of the available versions of the named Chart in
// this catalog.
// The result is sorted in descending order.
func (c Catalog) Versions(name string) []string {
	results := c.collect(func(chart *chart.Chart) string {
		if chart.Metadata.Name == name {
			return chart.Metadata.Version
		} else {
			return ""
		}
	})

	// Sort the result in descending order.
	// Example: 1.0.0, 1.0.0-beta, 0.9.0
	slices.SortFunc(results, func(a, b string) int {
		if res := semver.Compare("v"+b, "v"+a); res != 0 {
			return res
		} else {
			return cmp.Compare(b, a)
		}
	})

	return results
}

// AppVersions returns the list of the available appVersions of the named Chart
// in this catalog.
func (c Catalog) AppVersions(name string) []string {
	return c.collect(func(chart *chart.Chart) string {
		if chart.Metadata.Name == name {
			return chart.Metadata.AppVersion
		} else {
			return ""
		}
	})
}

// Append adds a new chart to the catalog. It ensures that the new chart has a
// valid metadata and a chart with the same name and version does not exist in
// the catalog.
func (c *Catalog) Append(chart *chart.Chart) {
	if chart.Metadata == nil {
		return
	}

	for _, i := range *c {
		if i.Metadata.Name == chart.Metadata.Name && i.Metadata.Version == chart.Metadata.Version {
			return
		}
	}

	*c = append(*c, chart)
}

// Populate uses the provided options to populate the existing Charts into the
// catalog.
//
// Currently it can only populate Charts from the local file system using a set
// of search paths and file name patterns. If a directory or an archive file in
// the specified search paths contain a chart it loads it and appends it to the
// catalog.
func (c *Catalog) Populate(options ...PopulateOption) error {
	cfg := defaultPopulateConfig(c)
	cfg.applyConfig(options)

	return cfg.populate()
}

/* Private */

func (c Catalog) collect(operator func(*chart.Chart) string) []string {
	col := map[string]bool{}

	for _, chart := range c {
		out := operator(chart)
		if out != "" {
			col[out] = true
		}
	}

	i := 0
	result := make([]string, len(col))

	for k := range col {
		result[i] = k
		i++
	}

	return result
}

func clone(in *chart.Chart) (*chart.Chart, error) {
	/*
	 *  This is a limited deep copy of a Chart. It only clones the values and
	 *  metadata of a Chart and does the same for its dependencies, including
	 *  transitive dependencies. As a result the reference to the dependencies
	 *  changes but all other attributes except values remain the same.
	 */
	out := *in

	if v, err := copystructure.Copy(out.Values); err != nil {
		return &out, err
	} else {
		out.Values = v.(map[string]interface{})
	}

	// Deep copy the whole metadata as .Metadata.Dependencies is modified during
	// rendering.
	if v, err := copystructure.Copy(out.Metadata); err != nil {
		return &out, err
	} else {
		out.Metadata = v.(*chart.Metadata)
	}

	depList := make([]*chart.Chart, 0, len(out.Dependencies()))

	for _, dep := range out.Dependencies() {
		if depCopy, err := clone(dep); err != nil {
			return &out, err
		} else {
			depList = append(depList, depCopy)
		}
	}

	out.SetDependencies(depList...)

	return &out, nil
}
