---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH를 통한 Git 지원
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

이 문서는 다양한 환경 및 플랫폼에서 SSH를 통한 Git 구성 가이드라인을 제공합니다.

## 개요 {#overview}

[GitLab Shell Helm 차트](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/)는 GitLab에 대한 Git SSH 액세스를 위해 구성된 SSH 서버를 제공합니다. 이 컴포넌트는 클러스터 외부의 포트 `22`에 노출되어야 합니다.

GitLab Operator는 `gitlab.gitlab-shell.enabled`가 `true`로 설정된 경우 `gitlab-shell`을 배포하며, 이는 기본 설정입니다.

대상 플랫폼에 따른 요구 사항을 요약하면 다음과 같습니다.

| SSH를 통한 Git이 필요하십니까? | Kubernetes                                                                                                    | OpenShift |
|------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| 아니요                           | 아래 NGINX Ingress 공급자 중 하나를 사용해야 합니다(Kubernetes에는 기본 제공 Ingress 공급자가 없습니다). | 아래 Ingress 공급자가 필요하지 않습니다. 기본 제공 Routes를 Ingress 공급자로 사용할 수 있습니다. |
| 예                          | 아래 NGINX Ingress 공급자 중 하나를 사용해야 합니다(Kubernetes에는 기본 제공 Ingress 공급자가 없습니다). | 아래 Ingress 공급자 중 하나를 사용해야 합니다. Routes는 포트 `22` 노출을 지원하지 않습니다. |

## Ingress 공급자 {#ingress-providers}

아래는 관련 참고 사항 및 플랫폼별 세부 정보와 함께 Ingress 공급자 목록입니다.

### NGINX-Ingress Helm 차트 {#nginx-ingress-helm-chart}

GitLab은 SSH를 통한 Git을 기본으로 지원하도록 수정된 NGINX 리소스를 배포하는 데 사용할 수 있는 [포크된 `NGINX-ingress` 차트](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork)를 유지 관리합니다.

이는 GitLab Operator 사용 시 기본 구성이며, GitLab CR의 `nginx-ingress.enabled={true,false}`로 제어됩니다. `false`로 설정하면 [외부 NGINX 인스턴스](https://docs.gitlab.com/charts/advanced/external-nginx/)를 사용할 수 있습니다.

이 Ingress 공급자는 Kubernetes와 OpenShift 모두에서 사용할 수 있습니다.

NGINX Ingress 공급자의 설치 옵션에 대한 자세한 내용은 [설치 문서](installation.md#ingress-controller)를 참조하세요.

### NGINX Ingress Operator {#nginx-ingress-operator}

기본 제공 NGINX-Ingress Helm 차트 포크의 대안으로, [NGINX Ingress Operator](https://github.com/nginxinc/nginx-ingress-operator)를 사용하여 `gitlab-shell`을 노출할 수 있습니다.

이 옵션에는 몇 가지 주의 사항이 있습니다.

- NGINX Inc.의 TransportServer/GlobalConfiguration 커스텀 리소스 정의는 기능 미리보기로 간주되며, 프로덕션 사용 시 주의를 권장합니다.
- NGINX Inc. Operator는 아직 비교적 초기 단계로, 버전 0.3.0에 불과합니다. 두 가지 유형의 보다 성숙한 Helm 차트에 비해 구성 옵션이 훨씬 적습니다.
- 이 옵션은 여전히 NGINX 서비스에서 포트 `22`를 수동으로 노출해야 합니다(NGINXIngressController CR에서는 구성할 수 없습니다).

보다 광범위한 연구 내용은 [#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916)에서 확인할 수 있습니다.

### OpenShift Routes {#openshift-routes}

OpenShift [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html)는 OpenShift 클러스터의 기본 제공 컴포넌트입니다. [Kubernetes Ingresses](https://kubernetes.io/docs/concepts/services-networking/ingress/)에 해당하는 OpenShift의 동등한 기능입니다.

OpenShift에 배포할 때 GitLab CR에서 `nginx-ingress.enabled=false`로 설정하고 OpenShift Routes가 외부 트래픽 흐름을 제어하도록 할 수 있습니다. GitLab Operator가 Ingress 오브젝트를 조정하면 OpenShift는 클러스터의 기본 도메인에 매핑되는 동등한 Route 오브젝트를 자동으로 생성합니다.

OpenShift Routes는 TCP 트래픽(포트 `22`의 SSH) 노출을 지원하지 않으므로 `gitlab-shell`을 통한 SSH 기반 Git에는 사용할 수 없습니다.

## 고려 사항 {#considerations}

아래는 Ingress 사용 시 고려해야 할 항목입니다.

### OpenShift에서 서드파티 Ingress 공급자 사용 {#using-a-third-party-ingress-provider-in-openshift}

OpenShift에서 서드파티 Ingress 컨트롤러를 사용하는 경우, OpenShift Ingress 컨트롤러가 일부 상황에서 서드파티 Ingress 컨트롤러와 충돌할 수 있습니다.

한 가지 예로, NGINX Ingress 컨트롤러가 Ingress `ADDRESS`를 NGINX 서비스의 외부 IP 주소로 설정하면, OpenShift Ingress 컨트롤러가 이를 클러스터의 기본 도메인으로 오버라이드하는 경우가 있습니다. 이는 DNS 구성과 충돌할 수 있으며, 특히 URL을 특정 NGINX 서비스에 매핑하는 A 레코드를 생성하기 위해 Ingress에 IP 주소가 있어야 하는 [external-dns](https://github.com/kubernetes-sigs/external-dns)와 같은 서비스를 사용할 때 문제가 됩니다. 이는 GitLab Operator CI 환경 내에서 발생하는 사례입니다.

이를 해결하기 위해 [OpenShift Ingress 컨트롤러를 패치](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26)하여 OpenShift 전용 네임스페이스만 관리하도록 설정함으로써, GitLab 전용 네임스페이스에서 생성한 Ingress가 의도치 않게 수정되지 않도록 합니다.
