---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Configure the GitLab Operator with GitLab Geo
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

The requirements, limitations and Geo configuration of the Operator
are the same as for the [GitLab chart](https://docs.gitlab.com/charts/advanced/geo/).

To deploy Geo sites with the Operator, apply the Helm chart values to
the GitLab custom resource by setting the `spec.chart.values`.

## Ingress class

The GitLab Operator does not come with an IngressClass of the secondary
[NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo).

This controller and IngressClass are only needed if:

1. You want to use a unified URL for GitLab Geo.
1. Your primary Ingress controller overrides incoming `X-Forwarded-For`
   headers (the bundled default NGINX chart does).

The process of creating the IngressClass depends on your installation method:

{{< tabs >}}

{{< tab title="Manifest and OLM" >}}

The IngressClass is not included in the default manifest and OLM release.

Create it manually:

```shell
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: <gitlab-name>-nginx-geo
spec:
  controller: k8s.io/ingress-nginx-geo
EOF
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

Enable the IngressClass by updating your values:

```yaml
nginx-ingress:
  geo:
    ingressClass:
      enabled: true
```

{{< /tab >}}

{{< /tabs >}}
