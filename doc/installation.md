---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Installation

WARNING:
The default Operator configuration is **not intended for production**.
The default resource creates an implementation where _all_ GitLab services are
deployed in the cluster, which is **not suitable for production workloads**.
For production deployments, you **must** follow the [Cloud Native Hybrid reference architectures](https://docs.gitlab.com/ee/administration/reference_architectures/#cloud-native-hybrid).

NOTE:
The GitLab Operator is under active development and is not yet suitable for production use. See our [`Minimal` to `Viable` Epic](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/23) for more information.

This document describes how to deploy the GitLab Operator via manifests in your Kubernetes or OpenShift cluster.

If using OpenShift, these steps are typically handled by the Operator Lifecycle Manager (OLM) once an operator bundle is published. However, to test the most recent operator images, users may need to install the operator using the deployment manifests available in the operator repository.

## Prerequisites

1. [Create or use an existing Kubernetes or OpenShift cluster](#cluster)
1. Install pre-requisite services and software
   - [Ingress controller](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [Metrics server](#metrics)
1. [Configure Domain Name Services](#configure-domain-name-services)

### Cluster

::Tabs

:::TabTitle Kubernetes

To create a traditional Kubernetes cluster, consider using [official tooling](https://kubernetes.io/docs/tasks/tools/) or your preferred method of installation.

The GitLab Operator supports the following Kubernetes versions:

- A cluster running Kubernetes 1.20 or newer is required for all components to work.
- 1.26 support is fully tested as of Operator 0.24.0.
The GitLab Operator supports the following Kubernetes versions:

| Operator Version | Minimum Kubernetes version | Maximum Kubernetes version | Partially tested Kubernetes version(s) |
|--|--|--|--|
| `0.24.0` | `1.20` | `1.26` | `1.27`, `1.28` |

The last column lists newer versions of Kubernetes that have undergone initial testing but are not yet fully validated. You can track progress toward support for new Kubernetes versions in [Epic 11331](https://gitlab.com/groups/gitlab-org/-/epics/11331).

The GitLab Operator aims to support new minor Kubernetes versions four months after their initial release.
We welcome any compatibility issues with releases newer than those listed above in our [issue tracker](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues).

Some GitLab features might not work on versions older than the versions listed above.

For some components, like the [agent for Kubernetes](https://docs.gitlab.com/ee/user/clusters/agent/#gitlab-agent-for-kubernetes-supported-cluster-versions) and [GitLab Charts](https://docs.gitlab.com/charts/installation/cloud/#supported-kubernetes-versions), GitLab might support different cluster versions.

:::TabTitle OpenShift

To create an OpenShift cluster, see the [OpenShift cluster setup documentation](developer/openshift_cluster_setup.md) for an example of how to create a _development environment_.

GitLab Operator supports OpenShift 4.10 through 4.13.

::EndTabs

Cluster nodes must use the x86-64 architecture.
Support for multiple architectures, including AArch64/ARM64, is under active development.
See [issue 2899](https://gitlab.com/gitlab-org/charts/gitlab/-/issues/2899) for more information.

### Ingress controller

An Ingress controller is required to provide external access to the application and secure communication between components.

The GitLab Operator deploys our [forked NGINX chart from the GitLab Helm Chart](https://docs.gitlab.com/charts/charts/nginx/) by default.

If you prefer to use an external Ingress controller, use [NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/) by the Kubernetes community to deploy an Ingress Controller. Follow the relevant instructions in the link based on your platform and preferred tooling. Take note of the Ingress class value for later (it typically defaults to `nginx`).
When configuring the GitLab CR, be sure to set `nginx-ingress.enabled=false` to disable the NGINX objects from the GitLab Helm Chart.

### TLS certificates

To create a certificate for the operator's Kubernetes webhook, [cert-manager](https://cert-manager.io) is used. You should
use [cert-manager](https://cert-manager.io) for the GitLab certificates as well.

Because the operator needs a certificate for the Kubernetes webhook, you can't use the cert-manager bundled with the GitLab Chart. Instead, install cert-manager before you install the operator.

To install cert-manager, see the [installation documentation](https://cert-manager.io/docs/installation/) for your platform and tooling.

Our codebase targets [cert-manager 1.6.1](https://cert-manager.io/v1.6-docs).

NOTE:
Because [cert-manager 1.6](https://github.com/jetstack/cert-manager/releases/tag/v1.6.0) removed some deprecated APIs, if you deploy cert-manager 1.6 or higher, you need at least GitLab Operator 0.4.

### Metrics

::Tabs

:::TabTitle Kubernetes

Install the [metrics server](https://github.com/kubernetes-sigs/metrics-server#installation) so the HorizontalPodAutoscalers can retrieve pod metrics.

:::TabTitle OpenShift

OpenShift ships with [Prometheus Adapter](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html) by default, so there is no manual action required here.

::EndTabs

### Configure Domain Name Services

You need an internet-accessible domain to which you can add a DNS record.

See our [networking and DNS documentation](https://docs.gitlab.com/charts/installation/tools.html#networking-and-dns) for more details on connecting your domain to the GitLab components. You use the configuration mentioned in this section when defining your GitLab custom resource (CR).

Ingress in OpenShift requires extra consideration. See our [notes on OpenShift Ingress](openshift_ingress.md) for more information.

## Installing the GitLab Operator

1. Deploy the GitLab Operator.

   ```shell
   GL_OPERATOR_VERSION=<your_desired_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
   PLATFORM=kubernetes # or "openshift"
   kubectl create namespace gitlab-system
   kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
   ```

   This command first deploys the service accounts, roles and role bindings used by the operator, and then the operator itself.

   By default, the Operator watches the namespace where it is deployed.
   To instead watch at the cluster scope, remove the `WATCH_NAMESPACE`
   environment variable from the Deployment in the manifest under:
   `spec.template.spec.containers[0].env` and re-run the `kubectl apply` command above.

   NOTE:
   Running the Operator at the cluster scope is considered experimental.
   See [issue #100](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/100) for more information.

   Experimental:
   Alternatively, deploy the GitLab Operator via Helm.

   ```shell
   helm repo add gitlab-operator https://gitlab.com/api/v4/projects/18899486/packages/helm/stable
   helm repo update
   helm install gitlab-operator gitlab-operator/gitlab-operator --create-namespace --namespace gitlab-system
   ```

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
       version: "X.Y.Z" # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/0.8.1/CHART_VERSIONS
       values:
         global:
           hosts:
             domain: example.com # use a real domain here
           ingress:
             configureCertmanager: true
         certmanager-issuer:
           email: youremail@example.com # use your real email address here
   ```

   For more details on configuration options to use under `spec.chart.values`,
   see the [GitLab Helm Chart documentation](https://docs.gitlab.com/charts/charts/).

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
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

  When the CR is reconciled (the status of the GitLab resource is `Running`), you can access GitLab in your browser at `https://gitlab.example.com`.

To log in you need to retrieve the initial root password for your deployment. See the [Helm Chart documentation](https://docs.gitlab.com/charts/installation/deployment.html#initial-login) for further instructions.

## Recommended next steps

After completing your installation, consider taking the
[recommended next steps](https://docs.gitlab.com/ee/install/next_steps.html),
including authentication options and sign-up restrictions.

## Uninstall the GitLab Operator

Follow the steps below to remove the GitLab Operator and its associated resources.

Items to note prior to uninstalling the operator:

- The operator does not delete the Persistent Volume Claims or Secrets when a GitLab instance is deleted.
- When deleting the Operator, the namespace where it is installed (`gitlab-system` by default) is not deleted automatically. This ensures that persistent volumes are not lost unintentionally.

### Uninstall an instance of GitLab

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

This removes the GitLab instance, and all associated objects except for Persistent Volume Claims as noted above).

### Uninstall the GitLab Operator

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

This deletes the Operator's resources, including the running Deployment of the Operator. This **does not** delete objects associated with a GitLab instance.

## Troubleshoot the GitLab Operator

Troubleshooting the Operator can be found in [troubleshooting.md](troubleshooting.md).
