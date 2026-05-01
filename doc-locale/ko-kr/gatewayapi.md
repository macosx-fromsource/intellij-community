---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Gateway API 및 Envoy Gateway 사용
---

{{< details >}}

- Tier: Free, 프리미엄, Ultimate
- Offering: GitLab Self-Managed
- Status: 베타

{{< /details >}}

> [!warning]
> Operator에서 Gateway API를 활성화하기 전에 GitLab 차트 [Gateway API 문서](https://docs.gitlab.com/charts/charts/globals/#gateway-api)에서
> 사용 가능한 구성 옵션 및 현재 제한 사항을 확인하세요.

Operator 2.10 및 GitLab 차트 9.7부터 GitLab은 Ingress 대신 [Gateway API](https://gateway-api.sigs.k8s.io/)를 사용하여 노출할 수 있습니다. 이는 [NGINX Ingress 지원 종료](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) 이후 Kubernetes 커뮤니티의 권장 사항을 따른 것입니다.

## 사전 요구 사항 {#prerequisites}

GitLab Operator는 GitLab 차트와 달리 Gateway API 컨트롤러를 번들로 제공하지 않습니다. Operator가 관리하는 GitLab 인스턴스를 Gateway API를 통해 노출하려면 먼저 [Envoy Gateway](https://gateway.envoyproxy.io/)와 같은 Gateway API 구현체를 설치해야 합니다.

Envoy Gateway를 사용하려는 경우 공식 Envoy Gateway Helm 차트를 사용한다면, Envoy Gateway 값에서 `config.envoyGateway.extensionsApi.enableEnvoyPatchPolicy=true`를 설정하여 EnvoyPatchPolicies 지원을 활성화해야 합니다.

certmanager로 TLS 인증서를 관리하려는 경우 [Gateway API](https://cert-manager.io/docs/usage/gateway/)에 맞게 구성하세요.
