---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Use Gateway API and Envoy Gateway
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed
- Status: Beta

{{< /details >}}

{{< alert type="warning" >}}

Before enabling Gateway API with the Operator check the GitLab chart [Gateway API documentation](https://docs.gitlab.com/charts/charts/globals/#gateway-api)
for details on the available configuration options and current limitations.

{{< /alert >}}

Starting with Operator 2.9 and GitLab chart 9.7, GitLab can be exposed by using [Gateway API](https://gateway-api.sigs.k8s.io/)
instead of Ingress. This follows the Kubernetes community recommendation after the
[NGINX Ingress retirement](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/).

## Prerequisites

The GitLab Operator does not bundle a Gateway API controller like GitLab chart does. Before exposing
a GitLab instance managed by the Operator through Gateway API, you must first install a Gateway API
implementation such as [Envoy Gateway](https://gateway.envoyproxy.io/).

If you want to use Envoy Gateway and you use the official Envoy Gateway Helm chart, make sure support
for EnvoyPatchPolicies is enabled by setting `config.envoyGateway.extensionsApi.enableEnvoyPatchPolicy=true`
in your Envoy Gateway values.

If you intend to manage TLS certificates with certmanager, make sure to configure it for
[Gateway API](https://cert-manager.io/docs/usage/gateway/).
