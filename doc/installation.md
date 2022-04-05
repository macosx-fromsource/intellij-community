# Installation

NOTE:
The GitLab Operator is under active development and is not yet suitable for production use. See our [`Minimal` to `Viable` Epic](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/39) for more information.

This document describes how to deploy the GitLab Operator via manifests in your Kubernetes or OpenShift cluster.

If using OpenShift, these steps normally are handled by OLM (the Operator Lifecycle Manager) once an operator is bundle published. However, to test the most recent operator images, users may need to install the operator using the deployment manifests available in the operator repository.

## Prerequisites

1. [Create or use an existing Kubernetes or OpenShift cluster](#cluster)
1. Install pre-requisite services and software
   - [Ingress controller](#ingress-controller)
   - [Certificate manager](#tls-certificates)
   - [Metrics server](#metrics)
1. [Configure Domain Name Services](#configure-domain-name-services)

### Cluster

#### Kubernetes

To create a traditional Kubernetes cluster, consider using [official tooling](https://kubernetes.io/docs/tasks/tools/) or your preferred method of installation.

GitLab Operator supports Kubernetes 1.19 through 1.21. Support for Kubernetes 1.22 is under active development - see [&6883](https://gitlab.com/groups/gitlab-org/-/epics/6883) for more information.

#### OpenShift

To create an OpenShift cluster, see the [OpenShift cluster setup docs](developer/openshift_cluster_setup.md) for an example of how to create a _development environment_.

GitLab Operator supports OpenShift 4.8 through 4.9. Support for Kubernetes 1.22 is under active development - see [&6883](https://gitlab.com/groups/gitlab-org/-/epics/6883) for more information.

### Ingress controller

An Ingress controller is required to provide external access to the application and secure communication between components.

The GitLab Operator will deploy our [forked NGINX chart from the GitLab Helm Chart](https://docs.gitlab.com/charts/charts/nginx/) by default.

If you prefer to use an external Ingress controller, we recommend [NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/) by the Kubernetes community to deploy an Ingress Controller. Follow the relevant instructions in the link based on your platform and preferred tooling. Take note of the Ingress class value for later (it typically defaults to `nginx`).

When configuring the GitLab CR, be sure to set `nginx-ingress.enabled=false` to disable the NGINX objects from the GitLab Helm Chart.

### TLS certificates

We recommend [Cert Manager](https://cert-manager.io/docs/installation/) to create certificates used to secure the GitLab and Registry URLs. Follow the relevant instructions in the link based on your platform and preferred tooling.

Our codebase currently targets Cert Manager 1.6.1.

NOTE:
Cert Manager [1.6](https://github.com/jetstack/cert-manager/releases/tag/v1.6.0) removed some deprecated APIs. As a result, if deploying Cert Manager >= 1.6, you will need GitLab Operator >= 0.4.

### Metrics

#### Kubernetes

Install the [metrics server](https://github.com/kubernetes-sigs/metrics-server#installation) so the HorizontalPodAutoscalers can retrieve pod metrics.

#### OpenShift

OpenShift ships with [Prometheus Adapter](https://docs.openshift.com/container-platform/4.8/monitoring/understanding-the-monitoring-stack.html#default-monitoring-components_understanding-the-monitoring-stack) by default, so there is no manual action required here.

### Configure Domain Name Services

You will need an internet-accessible domain to which you can add a DNS record.

See our [networking and DNS documentation](https://docs.gitlab.com/charts/installation/deployment.html#networking-and-dns) for more details on connecting your domain to the GitLab components. You will use the configuration mentioned in this section when defining your GitLab custom resource (CR).

## Installing the GitLab Operator

1. Deploy the GitLab Operator.

   ```shell
   GL_OPERATOR_VERSION=0.5.1 # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
   PLATFORM=kubernetes # or "openshift"
   kubectl create namespace gitlab-system
   kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
   ```

   This command first deploys the service accounts, roles and role bindings used by the operator, and then the operator itself.

   By default, the Operator will only watch the namespace where it is deployed.
   If you'd like it to watch at the cluster scope, then remove the `WATCH_NAMESPACE`
   environment variable from the Deployment in the manifest under:
   `spec.template.spec.containers[0].env` and re-run the `kubectl apply` command above.

   NOTE:
   Running the Operator at the cluster scope is considered experimental.
   See [issue #100](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/100) for more information.

1. Create a GitLab custom resource (CR).

   Create a new file named something like `mygitlab.yaml`.

   Here is an example of the content to put in this file:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: example
   spec:
     chart:
       version: "X.Y.Z" # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/0.5.1/CHART_VERSIONS
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
   see the [GitLab Helm Chart documentation](https://docs.gitlab.com/charts/charts).

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

  When the CR is reconciled (the status of the GitLab resource will be `Running`), you can access GitLab in your browser at `https://gitlab.example.com`.

## Uninstall the GitLab Operator

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
GL_OPERATOR_VERSION=0.5.1 # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

This will delete the Operator's resources, including the running Deployment of the Operator. This **will not** delete objects associated with a GitLab instance.

## Troubleshoot the GitLab Operator

Troubleshooting the Operator can be found in [troubleshooting.md](troubleshooting.md).
