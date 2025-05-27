---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Support for Git over SSH
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

This document provides configuration guidelines for Git over SSH on various environments/platforms.

## Overview

The [GitLab Shell Helm chart](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/) provides an SSH server configured for Git SSH access to GitLab. This component must be exposed outside of the cluster on port `22`.

The GitLab Operator deploys `gitlab-shell` when `gitlab.gitlab-shell.enabled` is set to `true`, which is the default setting.

To summarize the requirements based on the target platform:

| Do you require Git over SSH? | Kubernetes                                                                                                    | OpenShift |
|------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| No                           | You must use one of the NGINX Ingress providers below (Kubernetes does not have a built-in Ingress provider). | You do not need the Ingress providers below - can use built-in Routes as the Ingress provider. |
| Yes                          | You must use one of the NGINX Ingress providers below (Kubernetes does not have a built-in Ingress provider). | You must use one of the Ingress providers below - Routes do not support exposing port `22`. |

## Ingress providers

Below is a list of Ingress providers along with relevant notes and platform-specific details.

### NGINX-Ingress Helm Chart

GitLab maintains a [forked `NGINX-ingress` chart](https://docs.gitlab.com/charts/charts/nginx/fork/) that can be used to deploy NGINX resources that have been modified to support Git over SSH "out of the box".

This is the default configuration when using the GitLab Operator, and is controlled by `nginx-ingress.enabled={true,false}` in the GitLab CR. When set to `false`, you can use an [external NGINX instance](https://docs.gitlab.com/charts/advanced/external-nginx/).

This Ingress provider can be used on both Kubernetes and OpenShift.

More information on installation options for the NGINX Ingress provider is available in our [installation documentation](installation.md#ingress-controller).

### NGINX Ingress Operator

As an alternative to the built-in NGINX-Ingress Helm chart fork, the [NGINX Ingress Operator](https://github.com/nginxinc/nginx-ingress-operator) can be used to expose `gitlab-shell`.

There are some caveats with this option:

- NGINX Inc. TransportServer/GlobalConfiguration custom resource definitions are considered a feature preview and they recommend caution for production use.
- The NGINX Inc. Operator is still relatively young, only at version 0.3.0. It doesn't contain nearly as many configuration
  options as the more mature Helm Charts of either flavor.
- This option still requires _manually_ exposing port `22` on the NGINX Service (this is not configurable in the NGINXIngressController CR).

More extensive research is captured in [#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916).

### OpenShift Routes

OpenShift [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html) are a built-in component for OpenShift clusters. They are the OpenShift equivalent to [Kubernetes Ingresses](https://kubernetes.io/docs/concepts/services-networking/ingress/).

When deploying to OpenShift, you can set `nginx-ingress.enabled=false` in the GitLab CR and allow OpenShift Routes to control the flow of external traffic. When the GitLab Operator reconciles Ingress objects, OpenShift will automatically create an equivalent Route object that maps to the base domain of the cluster.

Note that OpenShift Routes do not support exposing TCP traffic (SSH on port `22`), and therefore cannot be used for Git over SSH via `gitlab-shell`.

## Considerations

Below are items to consider when working with Ingress.

### Using a third party Ingress provider in OpenShift

When using a third party Ingress controller on OpenShift, the OpenShift Ingress Controller can conflict with the third party Ingress controller in some cases.

One example is that the NGINX Ingress Controller will set an Ingress `ADDRESS` to the NGINX Service's external IP address, and then OpenShift Ingress Controller will override it with the base domain of the cluster. This can conflict with DNS configuration, especially when using a service like [external-dns](https://github.com/kubernetes-sigs/external-dns) which relies on the Ingress having an IP address so it can create A records to map the URL to that specific NGINX Service. This is the case within the GitLab Operator CI environment.

To work around it, we [patch the OpenShift Ingress Controller](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26) to only manage OpenShift-specific namespaces, ensuring that Ingresses we create in GitLab-specific namespaces are not modified undesirably.
