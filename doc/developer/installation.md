---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Installation

This document describes how to deploy the GitLab Operator via manifests in your Kubernetes or OpenShift cluster.

If using OpenShift, these steps normally are handled by OLM (the Operator Lifecycle Manager) once an operator is bundle published. However, to test the most recent operator images, users may need to install the operator using the deployment manifests available in the operator repository.

## Prerequisites

Please consult the "Prerequisites" section of the [installation](../installation.md#prerequisites) page.

## Installing the GitLab Operator

1. Clone the GitLab Operator repository to your local system.

   ```shell
   git clone https://gitlab.com/gitlab-org/cloud-native/gitlab-operator.git
   cd gitlab-operator
   ```

1. Deploy the GitLab Operator.

   ```shell
   kubectl create namespace gitlab-system
   task deploy_operator # or "task deploy_operator_openshift"
   ```

   This command first deploys the service accounts, roles and role bindings used by the operator, and then the operator itself.

   NOTE:
   By default, the Operator will only watch the namespace where it is deployed. If you would like it to watch at the cluster scope,
   modify [`deploy/chart/values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml) by setting `watchCluster` to `true`.

1. Create a GitLab custom resource (CR).

   Create a new file named something like `mygitlab.yaml`.

   Here is an example of the content to put in this file:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   spec:
     chart:
       version: "X.Y.Z" # select a version from the CHART_VERSIONS file in the root of this project
       values:
         global:
           hosts:
             domain: example.com # use a real domain here
           ingress:
             configureCertmanager: true
         certmanager-issuer:
           email: youremail@example.com # use your real email address here
   ```

   For more details on configuration options to use under `spec.chart.values`, see our [GitLab Helm Chart documentation](https://docs.gitlab.com/charts/).

1. Deploy a GitLab instance using your new GitLab CR.

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   This command sends your GitLab CR up to the cluster for the GitLab Operator to reconcile. You can watch the progress by tailing the logs from the controller pod:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   You can also list GitLab resources and check their status:

   ```shell
   kubectl get gitlabs -n gitlab-system
   ```

   When the CR is reconciled (the status of the GitLab resource will be `RUNNING`), you can access GitLab in your browser at `https://gitlab.example.com`.

## Cleanup

Certain operations like file removal under `config/` directory may not trigger rebuild/redeploy, in which cases one should employ:

```shell
task clean
```

This will remove all of the build artifacts and the install record.

## Uninstall

Follow the steps below to remove the GitLab Operator and its associated resources.

Items to note prior to uninstalling the operator:

- The operator does not delete the Persistent Volume Claims or Secrets when a GitLab instance is deleted.
- When deleting the Operator, the namespace where it is installed (`gitlab-system` by default) will not be deleted automatically. This is to ensure persistent volumes are not lost unintentionally.

### Uninstall an instance of GitLab

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

This will remove the GitLab instance, and all associated objects except for (PVCs as noted above).

### Uninstall the GitLab Operator

```shell
task delete_operator
```

This will delete the Operator's resources, including the running Deployment. It will not delete objects associated with a GitLab instance.
