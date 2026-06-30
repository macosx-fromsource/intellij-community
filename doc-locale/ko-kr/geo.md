---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Operator를 GitLab Geo와 함께 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab Self-Managed

{{< /details >}}

Operator의 요구 사항, 제한 사항 및 Geo 구성은 [GitLab chart](https://docs.gitlab.com/charts/advanced/geo/)와 동일합니다.

Operator를 사용하여 Geo 사이트를 배포하려면 Helm chart 값을 GitLab 사용자 정의 리소스에 적용하고 `spec.chart.values`를 설정하세요.

## Gateway API {#gateway-api}

[Envoy Gateway와 함께 Gateway API](gatewayapi.md)를 사용하는 경우, chart 문서를 따르는 것 외에 추가 작업이 필요하지 않습니다. Operator는 `global.geo.gatewayApi.additionalHostname` 구성을 적용하여 사이트 간 내부 통신을 활성화합니다.

## 수신 클래스 {#ingress-class}

> [!warning]
> NGINX Ingress는 GitLab chart 19.0부터 더 이상 사용되지 않으며 GitLab 20.0에서 제거될 예정입니다.
> 새로운 Geo 배포에는 [Envoy Gateway와 함께 Gateway API](gatewayapi.md)를 사용하세요.
> 기존 Geo 배포는 가능한 한 빨리 마이그레이션해야 합니다.

GitLab Operator는 보조 [NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo)의 IngressClass와 함께 제공되지 않습니다.

이 컨트롤러 및 IngressClass는 다음의 경우에만 필요합니다:

1. GitLab Geo에 통합 URL을 사용하려고 합니다.
1. 기본 Ingress 컨트롤러가 수신 `X-Forwarded-For` 헤더를 재정의합니다(번들 기본 NGINX chart가 그렇게 함).

IngressClass를 생성하는 프로세스는 설치 방법에 따라 다릅니다:

{{< tabs >}}

{{< tab title="Manifest and OLM" >}}

IngressClass는 기본 manifest 및 OLM 릴리스에 포함되지 않습니다.

수동으로 생성:

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

값을 업데이트하여 IngressClass를 활성화합니다:

```yaml
nginx-ingress:
  geo:
    ingressClass:
      enabled: true
```

{{< /tab >}}

{{< /tabs >}}
