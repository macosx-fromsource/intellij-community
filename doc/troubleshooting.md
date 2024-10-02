---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Troubleshooting the Operator

This document is a collection of notes and tips to assist in troubleshooting
the installation of the GitLab Operator and the deployment of a GitLab
instance from the GitLab custom resource.

## Installation problems

Troubleshooting the installation of the operator in a Kubernetes environment
is much like troubleshooting any other Kubernetes workload. After deploying
the operator manifest monitor the output of `kubectl describe` for the
operator Pod or `kubectl get events -n <namespace>`. This will indicate any
problems with retrieving the operator image or any other pre-condition for
starting the operator.

If the operator is starting up, but exits prematurely, examining the operator
logs can provide information for determining the cause for the Pod's
termination. This can be done with the following command:

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

Additionally, the operator depends on Cert Manager to create TLS
certificate for proper operation. The TLS certificate gets created as a
Secret and mounted as a volume on the operator Pod. Problems with obtaining
the TLS certificate can be found in the event log for the Namespace.

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

The next step would be to inspect the Cert Manager logs looking for issues
that indicate the failure in creating the TLS certificate.

### OpenShift specific problems

OpenShift has a more restrictive security model and as a result, the GitLab
operator needs to be installed with the cluster administrator account. The
developer accounts do not have the necessary privileges to allow the operator
function properly.

If [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) Pods are unable to provision due to invalid SCC parameters as described in [this issue](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762), the proper workaround is to update SCC from the repository to allow Ingress NGINX start in your OpenShift cluster:

1. Fetch the [latest OpenShift manifest](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases) for the GitLab Operator. You need `gitlab-operator-openshift-VERSION.yaml`
1. Extract SCC

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. Apply `scc.yaml` to your cluster:

   ```shell
   kubectl apply -f scc.yaml
   ```

Installing from a released manifest from the GitLab Operator repository's Releases page would not have this problem, since the SCC is included.
Related issues for objects not being supported in OperatorHubs:

- [Add support for IngressClass CR · Issue #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [Add support for OpenShift's SCC · Issue #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## Problems with deployment of GitLab instance

In addition to the information presented here, one should consult the
GitLab Helm chart [troubleshooting documentation](https://docs.gitlab.com/charts/troubleshooting/index.html).

### Core services not ready

The GitLab Operator relies on installing instances of Redis, PostgreSQL and
Gitaly. These are known as the core services. If after deploying a GitLab
customer resource there are an excessive number of operator log messages
stating that the core services are not ready, then it is one of these
services that is having problems becoming operational.

Specifically check the endpoints for each of these services to insure that
they are getting connected to the service's Pod. This is also a possible
indication that the cluster does not have enough resources to support the
GitLab instance and additional nodes should be added to the cluster.

Issue [#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)
has been created to track the reporting of which core service is stopping
the deployment of the GitLab instance.

### GitLab UI unreachable (Ingresses have no address and/or CertManager Challenges failing)

The GitLab Operator's installation manifest and Helm Chart use `gitlab` as the prefix
for all resource names by default unless `nameOverride` is specified in the Helm values.

As a result, the NGINX IngressClass will be named `gitlab-nginx`. If a release name other than
`gitlab` is specified in the GitLab custom resource under `metadata.name`, then the default
IngressClass name must be set explicitly under `global.ingress.class`:

For example: if `metadata.name` is set to `demo`, then set `global.ingress.class=gitlab-nginx`:

```yaml
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: demo
spec:
  chart:
    version: "X.Y.Z"
    values:
      global:
        ingress:
          # Use the correct IngressClass name.
          class: gitlab-nginx
```

Without this explicit setting, the Ingresses would attempt to find an Ingress named
`demo-nginx`, which does not exist.

### NGINX Ingress Controller pods missing

In an OpenShift environment the
[NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)
is used in place of OpenShift Routes for directing traffic to the GitLab
instance (both HTTPS and SSH). If you are having a problem with connecting
to the GitLab instance, first insure that there is a deployment for the
NGINX Ingress Controller.

If a deployment is present, check the `READY` column of the
`kubectl get deploy` output. If the `READY` status is reported back as
`0/0`, then inspect the output of
`kubectl get events -n <namespace> | grep -i nginx` looking for messages
that state that the Security Context Constraint (SCC) has been violated.

This is an indication that the NGINX RBAC resources for OpenShift were
not deployed. The operator manifest for OpenShift should be reapplied with
the following command:

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

After the manifest has been applied, it may be necessary to delete the
Ingress controller Deployment to acquire the SCC properly and allow the
Ingress controller to create the Pods correctly.

### Horizontal pod autoscalers are not scaling

If it is found that the horizontal pod autoscalers (HPA) do not scale the
number of pods according to traffic load, then check for an installation
of the Metrics Server. In a Kubernetes cluster the Metrics Server is an
additional component that needs to be installed. The installation process
can be found in the [installation documentation](installation.md#metrics).

An OpenShift cluster has a built in Metrics Server and as a result the
HPAs should operate correctly.

### Restoring data when PersistentVolumeClaim configuration changes

When working with components such as MinIO for data persistence, it may sometimes be necessary to reconnect
to a previous PersistentVolume.

For example, [!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)
replaced the Operator-defined MinIO components with the MinIO components from the GitLab Helm Charts. As part of
this change, the object names changed, including the PersistentVolumeClaim. As a result, it was necessary for anyone
using the Operator-bundled MinIO instance to take extra steps to reconnect to the previous PersistentVolume containing
the persisted data.

After upgrading to GitLab Operator `0.6.4`, complete the following steps to connect a new PersistentVolumeClaim to a previous PersistentVolume:

1. Delete the `$RELEASE_NAME-minio-secret` Secret. The contents of the Secret will change with `0.6.4` upgrade, but the Secret name will not.
1. Edit the previous MinIO PersistentVolume, changing `.spec.persistentVolumeReclaimPolicy` from `Delete` to `Retain`.
1. Delete the previous MinIO StatefulSet, `$RELEASE_NAME-minio`.
1. Remove `.spec.ClaimRef` from the previous MinIO PersistentVolume to dissociate it from the previous MinIO PersistentVolumeClaim.
1. Delete the previous MinIO PersistentVolumeClaim, `export-gitlab-minio-0`.
1. Confirm the previous PersistentVolume status is now `Available`.
1. Set the following value in the GitLab custom resource: `minio.persistence.volumeName=<previous PersistentVolume name>`.
1. Apply the GitLab custom resource.
1. Delete the new MinIO PersistentVolumeClaim (and MinIO pod, so that the PersistentVolumeClaim is unbound and can be deleted). The Operator will recreate
   the PersistentVolumeClaim. This is required because the `.spec` field is immutable.
1. Confirm that the previous MinIO PersistentVolume is now bound to new MinIO PersistentVolumeClaim.
1. Confirm that data is restored by navigating in the GitLab UI to issues, artifacts, etc.

For more information on reconnecting to previous PersistentVolumes, see our
[persistent volumes documentation](https://docs.gitlab.com/charts/advanced/persistent-volumes/).

As a reminder, the bundled MinIO instance is [not recommended for production use](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart).

### Configure multiple database connections

In GitLab 16.0, GitLab defaults to using two database connections that point to the same PostgreSQL database.

If you wish to switch back to single database connection, refer to
[configuring multiple database connections](https://docs.gitlab.com/charts/charts/globals.html#configure-multiple-database-connections).

### Disabling or Renaming components

While renaming and disabling of resources is possible via changes to `nameOverride` and combination of various `*.enable: false` values, GitLab Operator does not automatically remove Kubernetes resources that are no longer needed. As
a result, any of the above operations would require manual
management of leftover resources.

Deleting an instance of the GitLab custom resource, however, will remove all resources associated with that
instance as expected.

Issue [!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889) has been
created to keep track of this.
