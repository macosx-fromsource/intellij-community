---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH를 통한 Git 지원
---

{{< details >}}

- 계층: Free, Premium, Ultimate
- 제품: GitLab Self-Managed

{{< /details >}}

[GitLab Shell Helm 차트](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/)는 GitLab에 대한 Git SSH 액세스를 위해 구성된 SSH 서버를 제공합니다. 이 구성 요소는 포트 `22`의 클러스터 외부에 노출되어야 합니다.

GitLab Operator는 `gitlab.gitlab-shell.enabled`이(가) `true`로 설정되어 있을 때 `gitlab-shell`을(를) 배포하며, 이것이 기본 설정입니다.

다음 방법 중 하나를 사용하여 SSH를 통한 Git을 노출하세요:

| 방법 | Kubernetes | OpenShift | 참고 사항 |
|:-----------------|:-----------|:---------------|:------|
| Gateway API | 지원됨 | 지원됨 | TCPRoute를 사용하는 Kubernetes Gateway API 표준을 활용하는 권장 최신 방식입니다. Envoy Gateway를 권장합니다. 요구 사항을 충족하는 경우 [다른 제공자](https://docs.gitlab.com/charts/advanced/gateway-api/#using-an-external-gateway-api-provider)도 사용할 수 있습니다. |
| NGINX Ingress | 더 이상 사용되지 않음 | 더 이상 사용되지 않음 | 기존 방식(포트 22 노출 필요). 번들 컨트롤러 대신 [외부 NGINX 컨트롤러](https://docs.gitlab.com/charts/advanced/external-ingress/)를 사용할 수 있습니다. |
| OpenShift Routes | 해당 없음 | SSH 미지원 | Routes는 TCP 트래픽(포트 22)을 지원하지 않습니다. |

## Envoy Gateway를 사용하는 Gateway API {#gateway-api-with-envoy-gateway}

GitLab은 기존 Ingress 리소스 대신 [Gateway API](https://gateway-api.sigs.k8s.io/)를 사용하여 노출할 수 있습니다.
이 방법은 `TCPRoute` 리소스를 통해 SSH를 통한 Git의 TCP 라우팅을 기본적으로 지원하므로 새 배포에 권장됩니다.

사전 요구 사항:

- GitLab Operator 2.10 이상.
- GitLab 차트 9.7 이상.

Gateway API는 Kubernetes와 OpenShift 클러스터 모두에서 작동합니다. 자세한 구성 지침 및 사전 요구 사항은
[Gateway API 및 Envoy Gateway 문서](gatewayapi.md)를 참조하세요.

`global.gatewayApi.enabled: true`로 Gateway API를 활성화하면 `gitlab-shell`이 자동으로 `TCPRoute` 리소스를 통해 노출되어 포트 `22`의 TCP 트래픽을 GitLab Shell 서비스로 라우팅합니다.

## NGINX Ingress {#nginx-ingress}

> [!warning]
> NGINX Ingress는 GitLab 차트 19.0부터 더 이상 사용되지 않으며 GitLab 20.0에서 제거될 예정입니다.
> 새 배포에는 [Envoy Gateway를 사용하는 Gateway API](#gateway-api-with-envoy-gateway)를 사용하세요.

GitLab Operator는 Kubernetes와 OpenShift 모두에서 NGINX Ingress 컨트롤러를 지원합니다. NGINX Ingress를 사용할 때는
SSH를 통한 Git을 활성화하기 위해 NGINX Service에서 포트 `22`를 노출해야 합니다.

GitLab은 SSH를 통한 Git을 지원하도록 구성된 NGINX 리소스를 배포하는 데 사용할 수 있는 [포크된 `NGINX-ingress` 차트](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork)를 유지 관리합니다.
이 차트는 더 이상 사용되지 않으며 지원되지 않지만, 기존 배포에서는 계속 사용할 수 있습니다.

NGINX Ingress는 GitLab CR의 `nginx-ingress.enabled={true,false}`에 의해 제어됩니다. `false`로 설정하면 [외부 NGINX 인스턴스](https://docs.gitlab.com/charts/advanced/external-nginx/)를 사용할 수 있습니다.

## OpenShift Routes {#openshift-routes}

OpenShift [Routes](https://docs.openshift.com/container-platform/4.10/networking/routes/route-configuration.html)는 OpenShift 클러스터의 기본 제공 Ingress 솔루션입니다. GitLab CR에서 `nginx-ingress.enabled=false`를 설정하여 NGINX Ingress를 비활성화하면 OpenShift가 Operator에서 생성한 Ingress 객체를 자동으로 동등한 Route 객체로 변환합니다.

OpenShift Routes는 TCP 트래픽(포트 `22`)을 노출하는 것을 지원하지 않으므로, OpenShift Routes를 사용할 때는 SSH를 통한 Git이 지원되지 않습니다.
OpenShift에서 SSH를 통한 Git이 필요한 경우 다음 중 하나를 선택하세요:

- Envoy Gateway를 사용하는 Gateway API를 사용하세요. GitLab CR에서 `global.gatewayApi.enabled=true`를 설정하세요.
- NGINX Ingress 컨트롤러를 사용하세요. `nginx-ingress.enabled=true`(기본값)를 설정하세요.

OpenShift의 Ingress 옵션에 대한 자세한 내용은 [OpenShift의 Ingress](openshift_ingress.md)를 참조하세요.

## 고려 사항 {#considerations}

다음은 Ingress를 사용할 때 고려해야 할 항목들입니다.

### OpenShift에서 제3자 Ingress 제공자 사용 {#using-a-third-party-ingress-provider-in-openshift}

OpenShift에서 제3자 Ingress 컨트롤러를 사용할 때, OpenShift Ingress 컨트롤러는 경우에 따라 제3자 Ingress 컨트롤러와 충돌할 수 있습니다.

한 가지 예는 NGINX Ingress 컨트롤러가 Ingress `ADDRESS`을(를) NGINX Service의 외부 IP 주소로 설정하고, 그러면 OpenShift Ingress 컨트롤러가 이를 클러스터의 기본 도메인으로 재정의한다는 것입니다. 이는 DNS 구성과 충돌할 수 있으며, 특히 [external-dns](https://github.com/kubernetes-sigs/external-dns)와 같은 서비스를 사용할 때 문제가 됩니다. 이 서비스는 Ingress가 IP 주소를 가지고 있어야 하므로 URL을 특정 NGINX Service로 매핑하기 위한 A 레코드를 생성할 수 있습니다. 이는 GitLab Operator CI 환경 내에서의 경우입니다.

이를 해결하기 위해 [OpenShift Ingress 컨트롤러를 패치](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26)하여 OpenShift 관련 네임스페이스만 관리하도록 하고, GitLab 관련 네임스페이스에서 만든 Ingress가 원치 않게 수정되지 않도록 합니다.
