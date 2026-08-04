package kubetests

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube"
	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/support/kube/apply"
)

var _ = Describe("ApplyObject", func() {
	It("uses strategic merge patch meta", func() {
		obj := ReadObject("apply/deployment-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		/* wait for the change to be populated */
		d := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d)).Should(Succeed())
		g := d.Generation

		obj = ReadObject("apply/deployment-2")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUpdated))

		d = &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d, g+1)).Should(Succeed())

		Expect(d.ObjectMeta.Generation).To(BeNumerically(">", g))
		Expect(d.Spec.Template.Spec.Volumes).To(HaveLen(3))
		Expect(d.Spec.Template.Spec.Volumes[2].Name).To(Equal("dummy"))
		Expect(d.Spec.Template.Spec.Containers[0].VolumeMounts).To(HaveLen(3))
		Expect(d.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("dummy"))

		Eventually(DeleteObject(d)).Should(Succeed())
	})

	It("patches custom resources whose Go type is registered in the scheme", func() {
		/* The kube-apiserver only accepts strategic merge patch for built-in
		   types. A custom resource must be patched with a JSON merge patch even
		   when its Go type is known, otherwise the server rejects the request
		   with 415 Unsupported Media Type. */
		obj := ReadObject("apply/servicemonitor-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		sm := &monitoringv1.ServiceMonitor{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(sm)).Should(Succeed())
		Expect(sm.Spec.JobLabel).To(Equal("test-job"))

		obj = ReadObject("apply/servicemonitor-2")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUpdated))

		Eventually(func() (string, error) {
			if err := GetObject(sm)(); err != nil {
				return "", err
			}

			return sm.Spec.JobLabel, nil
		}).Should(Equal("changed-job"))

		Eventually(DeleteObject(sm)).Should(Succeed())
	})

	It("reports an error when the server rejects the patch", func() {
		obj := ReadObject("apply/deployment-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		d := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d)).Should(Succeed())

		/* An immutable field: the server rejects the patch. */
		obj = ReadObject("apply/deployment-2")
		deployment, ok := obj.(*appsv1.Deployment)
		Expect(ok).To(BeTrue())

		deployment.Spec.Selector.MatchLabels = map[string]string{"app": "not-test"}

		outcome, err := kube.ApplyObject(obj, apply.WithManager(Manager))
		Expect(err).To(HaveOccurred())
		Expect(outcome).To(Equal(kube.ObjectUnchanged))

		Eventually(DeleteObject(d)).Should(Succeed())
	})

	It("patches the object when its last configuration is unknown", func() {
		obj := ReadObject("apply/deployment-1")
		Eventually(CreateObject(obj)).Should(Succeed())

		/* wait for the change to be populated */
		d := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d)).Should(Succeed())
		g := d.Generation

		obj = ReadObject("apply/deployment-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUpdated))

		/* wait for the change to be populated */
		d = &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d, g+1)).Should(Succeed())

		Expect(d.ObjectMeta.Generation).To(BeNumerically(">", g))
		Expect(d.Annotations).To(HaveKey(corev1.LastAppliedConfigAnnotation))
		Expect(d.Annotations[corev1.LastAppliedConfigAnnotation]).NotTo(BeEmpty())

		Eventually(DeleteObject(d)).Should(Succeed())
	})

	It("does not patch the object when it is not changed", func() {
		obj := ReadObject("apply/job-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		/* wait for the change to be populated */
		j := &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(j)).Should(Succeed())
		g := j.Generation

		obj = ReadObject("apply/job-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUnchanged))

		Expect(j.ObjectMeta.Generation).To(Equal(g))

		Eventually(DeleteObject(obj)).Should(Succeed())
	})

	It("does not modify the object when it is not semantically changed", func() {
		obj := ReadObject("apply/deployment-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		/* wait for the change to be populated */
		d := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d)).Should(Succeed())
		g := d.Generation

		obj = ReadObject("apply/deployment-1")
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUnchanged))

		d = &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d, g)).Should(Succeed())

		Expect(d.ObjectMeta.Generation).To(Equal(g))

		Eventually(DeleteObject(d)).Should(Succeed())
	})

	It("can work with unstructured objects", func() {
		obj := ReadObject("apply/deployment-1", UnstructuredYAMLCodec)
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectCreated))

		/* wait for the change to be populated */
		d := &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d)).Should(Succeed())
		g := d.Generation

		obj = ReadObject("apply/deployment-2", UnstructuredYAMLCodec)
		Expect(
			kube.ApplyObject(obj, apply.WithManager(Manager)),
		).To(Equal(kube.ObjectUpdated))

		d = &appsv1.Deployment{
			ObjectMeta: v1.ObjectMeta{
				Name:      obj.GetName(),
				Namespace: obj.GetNamespace(),
			},
		}
		Eventually(GetObject(d, g+1)).Should(Succeed())

		Expect(d.ObjectMeta.Generation).To(BeNumerically(">", g))
		Expect(d.Spec.Template.Spec.Volumes).To(HaveLen(3))
		Expect(d.Spec.Template.Spec.Volumes[2].Name).To(Equal("dummy"))

		Eventually(DeleteObject(obj)).Should(Succeed())
	})

	/*
	 * Testing unregistered types has proven to be difficult here. These types
	 * must be recognized by the mock Kubernetes API Server but not registered
	 * in the scheme.
	 */
})
