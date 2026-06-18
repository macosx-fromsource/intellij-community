---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Support for Git over SSH
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

The [GitLab Shell Helm chart](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/) provides an SSH server
configured for Git SSH access to GitLab. This component must be exposed outside of the cluster on port `22`.

The GitLab Operator deploys `gitlab-shell` when `gitlab.gitlab-shell.enabled` is set to `true`, which is the default setting.

Expose Git over SSH by using one of the following methods:

| Method           | Kubernetes | OpenShift      | Notes |
|:-----------------|:-----------|:---------------|:------|
| Gateway API      | Supported  | Supported      | Recommended modern approach using Kubernetes Gateway API standard with TCPRoute. Envoy Gateway is recommended. [Other providers](https://docs.gitlab.com/charts/advanced/gateway-api/#using-an-external-gateway-api-provider) might work if they meet the requirements. |
| NGINX Ingress    | Deprecated | Deprecated     | Traditional approach (requires port 22 exposure). [External NGINX controllers](https://docs.gitlab.com/charts/advanced/external-ingress/) can be used instead of the bundled one. |
| OpenShift Routes | N/A        | No SSH support | Routes do not support TCP traffic (port 22). |

## Gateway API with Envoy Gateway

GitLab can be exposed using [Gateway API](https://gateway-api.sigs.k8s.io/) instead of traditional Ingress resources.
This method is recommended for new deployments because it natively supports TCP routing for Git over SSH through
`TCPRoute`
resources.

Prerequisites:

- GitLab Operator 2.10 or later.
- GitLab chart 9.7 or later.

Gateway API works on both Kubernetes and OpenShift clusters. For detailed configuration instructions and prerequisites,
see the [Gateway API and Envoy Gateway documentation](gatewayapi.md).

When Gateway API is enabled with `global.gatewayApi.enabled: true`, `gitlab-shell` is automatically exposed through a
`TCPRoute` resource that routes TCP traffic on port `22` to the GitLab Shell service.

## NGINX Ingress

> [!warning]
> NGINX Ingress is deprecated as of GitLab chart 19.0 and will be removed in GitLab 20.0.
> Use [Gateway API with Envoy Gateway](#gateway-api-with-envoy-gateway) for new deployments.

The GitLab Operator supports NGINX Ingress Controller on both Kubernetes and OpenShift. When using NGINX Ingress,
port `22` must be exposed on the NGINX Service to enable Git over SSH.

GitLab maintains a [forked `NGINX-ingress` chart](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork)
that can be used to deploy NGINX resources configured to support Git over SSH.
This chart is deprecated and unsupported, but remains available for existing deployments.

NGINX Ingress is controlled by `nginx-ingress.enabled={true,false}` in the GitLab CR. When set to `false`, you can use an
[external NGINX instance](https://docs.gitlab.com/charts/advanced/external-nginx/).

## OpenShift Routes

OpenShift [Routes](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html) are a
built-in Ingress solution for OpenShift clusters. When you disable NGINX Ingress by setting `nginx-ingress.enabled=false`
in the GitLab CR, OpenShift automatically converts the Ingress objects created by the Operator into equivalent Route objects.

Git over SSH is not supported when using OpenShift Routes because Routes do not support exposing TCP traffic (port `22`).
If you require Git over SSH on OpenShift, either:

- Use Gateway API with Envoy Gateway. Set `global.gatewayApi.enabled=true` in your GitLab CR.
- Use the NGINX Ingress Controller. Set `nginx-ingress.enabled=true` (the default).

For more information on Ingress options in OpenShift, see the [Ingress in OpenShift](openshift_ingress.md).

## Considerations

Below are items to consider when working with Ingress.

### Using a third party Ingress provider in OpenShift

When using a third party Ingress controller on OpenShift, the OpenShift Ingress Controller can conflict with the third party Ingress controller in some cases.

One example is that the NGINX Ingress Controller will set an Ingress `ADDRESS` to the NGINX Service's external IP address, and then OpenShift Ingress Controller will override it with the base domain of the cluster. This can conflict with DNS configuration, especially when using a service like [external-dns](https://github.com/kubernetes-sigs/external-dns) which relies on the Ingress having an IP address so it can create A records to map the URL to that specific NGINX Service. This is the case within the GitLab Operator CI environment.

To work around it, we [patch the OpenShift Ingress Controller](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26) to only manage OpenShift-specific namespaces, ensuring that Ingresses we create in GitLab-specific namespaces are not modified undesirably.
