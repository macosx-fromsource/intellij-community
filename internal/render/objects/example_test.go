package objects

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// exampleObject builds a minimal rendered object for the examples.
func exampleObject(kind, name, component string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       kind,
	}}

	obj.SetName(name)
	obj.SetLabels(map[string]string{"app": component})

	return obj
}

// ExamplePartition splits one render into the workloads a zero-downtime
// upgrade phase gates and everything else.
func ExamplePartition() {
	rendered := []*unstructured.Unstructured{
		exampleObject("Deployment", "gitlab-webservice-default", "webservice"),
		exampleObject("Deployment", "gitlab-sidekiq-all-in-1-v2", "sidekiq"),
		exampleObject("Job", "gitlab-migrations", "migrations"),
		exampleObject("Service", "gitlab-webservice-default", "webservice"),
	}

	gated, rest := Partition(rendered, And(
		ByKind("Deployment"),
		Or(ByComponent("webservice"), ByComponent("sidekiq")),
	))

	for _, obj := range gated {
		fmt.Println("gated:", obj.GetName())
	}

	for _, obj := range rest {
		fmt.Println("rest:", obj.GetName())
	}

	// Output:
	// gated: gitlab-webservice-default
	// gated: gitlab-sidekiq-all-in-1-v2
	// rest: gitlab-migrations
	// rest: gitlab-webservice-default
}

// ExampleUpsertInitContainerEnv sets the schema-version bypass on a rendered
// Deployment, the way the zero-downtime upgrade releases held workloads.
func ExampleUpsertInitContainerEnv() {
	deployment := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "gitlab-webservice-default"},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"initContainers": []interface{}{
						map[string]interface{}{"name": "dependencies"},
					},
				},
			},
		},
	}}

	prepared := deployment.DeepCopy()

	if err := UpsertInitContainerEnv(prepared, "dependencies", "BYPASS_SCHEMA_VERSION", "true"); err != nil {
		fmt.Println("mutation failed:", err)
		return
	}

	env, _, _ := unstructured.NestedSlice(prepared.Object, "spec", "template", "spec", "initContainers")
	fmt.Println(env[0].(map[string]interface{})["env"])

	// Output:
	// [map[name:BYPASS_SCHEMA_VERSION value:true]]
}
