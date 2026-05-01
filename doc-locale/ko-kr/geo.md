---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Operator를 GitLab Geo와 함께 구성하기
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

Operator의 요구 사항, 제한 사항 및 Geo 구성은 [GitLab 차트](https://docs.gitlab.com/charts/advanced/geo/)와 동일합니다.

Operator를 사용하여 Geo 사이트를 배포하려면 `spec.chart.values`를 설정하여 GitLab 커스텀 리소스에 Helm 차트 값을 적용하세요.

## Ingress 클래스 {#ingress-class}

GitLab Operator에는 세컨더리 [NGINX Ingress](https://docs.gitlab.com/charts/charts/nginx/#gitlab-geo)의 IngressClass가 포함되어 있지 않습니다.

이 컨트롤러와 IngressClass는 다음 경우에만 필요합니다:

1. GitLab Geo에 통합 URL을 사용하려는 경우
1. 기본 Ingress 컨트롤러가 수신되는 `X-Forwarded-For` 헤더를 오버라이드하는 경우(번들로 제공되는 기본 NGINX 차트가 이에 해당합니다)

IngressClass 생성 방법은 설치 방법에 따라 다릅니다:

{{< tabs >}}

{{< tab title="Manifest and OLM" >}}

IngressClass는 기본 매니페스트 및 OLM 릴리즈에 포함되어 있지 않습니다.

수동으로 생성하세요:

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

값을 업데이트하여 IngressClass를 활성화하세요:

```yaml
nginx-ingress:
  geo:
    ingressClass:
      enabled: true
```

{{< /tab >}}

{{< /tabs >}}
