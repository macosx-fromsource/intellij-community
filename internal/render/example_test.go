package render

import (
	"fmt"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support"
)

// ExampleRender templates the fixture chart and lists the rendered objects in
// their deterministic order: known kinds follow the Helm install order, and
// unknown kinds, such as the foreign custom resource, sort after them.
func ExampleRender() {
	values := support.Values{}
	_ = values.SetValue("foreignCR.create", true)

	result, err := Render(Request{
		ChartPath:   "testdata/chart/test",
		ReleaseName: "myrel",
		Namespace:   "example",
		Values:      values,
	})
	if err != nil {
		fmt.Println("render failed:", err)
		return
	}

	for _, obj := range result.Objects {
		fmt.Printf("%s %s\n", obj.GetKind(), obj.GetName())
	}

	// Output:
	// ServiceAccount myrel-test
	// Secret myrel-test
	// ConfigMap myrel-test
	// Service myrel-test
	// Deployment myrel-test
	// ForeignResource myrel-test
}

// ExampleResult_HooksFor selects the hooks of one event in the Helm execution
// order: ascending weight, then object name.
func ExampleResult_HooksFor() {
	result, err := Render(Request{
		ChartPath:   "testdata/chart/hooked",
		ReleaseName: "myrel",
		Namespace:   "example",
	})
	if err != nil {
		fmt.Println("render failed:", err)
		return
	}

	for _, hook := range result.HooksFor("pre-upgrade") {
		fmt.Printf("%d %s %s %v\n", hook.Weight, hook.Object.GetKind(), hook.Object.GetName(), hook.DeletePolicies)
	}

	// Output:
	// -5 ServiceAccount myrel-hooked-migrate []
	// 5 Job myrel-hooked-migrate [before-hook-creation hook-succeeded]
}

// ExampleRender_warnings shows how documents that cannot become Kubernetes
// objects surface as typed warnings instead of failing the render.
func ExampleRender_warnings() {
	values := support.Values{}
	_ = values.SetValue("noTypeMeta.create", true)

	result, err := Render(Request{
		ChartPath:   "testdata/chart/hooked",
		ReleaseName: "myrel",
		Values:      values,
	})
	if err != nil {
		fmt.Println("render failed:", err)
		return
	}

	for _, warning := range result.Warnings {
		fmt.Println(warning.Reason)
	}

	// Output:
	// MissingTypeMeta
}
