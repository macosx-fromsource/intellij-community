---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH를 통한 Git 지원
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제품:  GitLab Self-Managed

{{< /details >}}

이 문서는 다양한 환경/플랫폼에서 SSH를 통한 Git에 대한 구성 지침을 제공합니다.

## 개요 {#overview}

[GitLab Shell Helm 차트](https://docs.gitlab.com/charts/charts/gitlab/gitlab-shell/)는 GitLab에 대한 Git SSH 액세스를 위해 구성된 SSH 서버를 제공합니다. 이 구성 요소는 포트 `22`의 클러스터 외부에 노출되어야 합니다.

GitLab Operator는 `gitlab-shell`을(를) 배포합니다. `gitlab.gitlab-shell.enabled`이(가) `true`로 설정되어 있으며, 이것이 기본 설정입니다.

대상 플랫폼에 따른 요구 사항을 요약하면 다음과 같습니다:

| Git over SSH가 필요합니까? | Kubernetes                                                                                                    | OpenShift |
|------------------------------|---------------------------------------------------------------------------------------------------------------|-----------|
| 아니요                           | 아래의 NGINX Ingress 제공자 중 하나를 사용해야 합니다(Kubernetes에는 기본 제공 Ingress 제공자가 없음). | 아래의 Ingress 제공자가 필요하지 않으며, 기본 제공 Routes를 Ingress 제공자로 사용할 수 있습니다. |
| 예                          | 아래의 NGINX Ingress 제공자 중 하나를 사용해야 합니다(Kubernetes에는 기본 제공 Ingress 제공자가 없음). | 아래의 Ingress 제공자 중 하나를 사용해야 합니다. Routes는 포트 `22`을(를) 노출하는 것을 지원하지 않습니다. |

## Ingress 제공자 {#ingress-providers}

다음은 Ingress 제공자 목록과 관련 참고 사항 및 플랫폼별 세부 사항입니다.

### NGINX-Ingress Helm 차트 {#nginx-ingress-helm-chart}

GitLab은 [포크된 `NGINX-ingress` 차트](https://docs.gitlab.com/charts/charts/nginx/#adjustments-to-the-nginx-fork)를 유지 관리하며, 이를 사용하여 Git over SSH를 "기본적으로" 지원하도록 수정된 NGINX 리소스를 배포할 수 있습니다.

이것은 GitLab Operator를 사용할 때 기본 구성이며, GitLab CR의 `nginx-ingress.enabled={true,false}`에 의해 제어됩니다. `false`로 설정하면 [외부 NGINX 인스턴스](https://docs.gitlab.com/charts/advanced/external-nginx/)를 사용할 수 있습니다.

이 Ingress 제공자는 Kubernetes와 OpenShift 모두에서 사용할 수 있습니다.

NGINX Ingress 제공자의 설치 옵션에 대한 자세한 정보는 [설치 문서](installation.md#ingress-controller)에서 확인할 수 있습니다.

### NGINX Ingress Operator {#nginx-ingress-operator}

기본 제공 NGINX-Ingress Helm 차트 포크의 대안으로, [NGINX Ingress Operator](https://github.com/nginxinc/nginx-ingress-operator)를 사용하여 `gitlab-shell`을(를) 노출할 수 있습니다.

이 옵션에는 다음과 같은 주의 사항이 있습니다:

- NGINX Inc. TransportServer/GlobalConfiguration 사용자 정의 리소스 정의는 기능 미리보기로 간주되며, 프로덕션 사용에 주의할 것을 권장합니다.
- NGINX Inc. Operator는 여전히 상대적으로 새로운 버전 0.3.0입니다. 이는 더 성숙한 두 가지 형태의 Helm 차트만큼 많은 구성 옵션을 포함하지 않습니다.
- 이 옵션은 여전히 NGINX Service의 포트 `22`를 수동으로 노출해야 하며, 이는 NGINXIngressController CR에서 구성할 수 없습니다.

더 광범위한 연구 내용은 [\#58](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/58#note_585883916)에 캡처되어 있습니다.

### OpenShift Routes {#openshift-routes}

OpenShift [Routes](https://docs.openshift.com/container-platform/3.4/architecture/core_concepts/routes.html)는 OpenShift 클러스터의 기본 제공 구성 요소입니다. 이는 [Kubernetes Ingresses](https://kubernetes.io/docs/concepts/services-networking/ingress/)와 동등합니다.

OpenShift에 배포할 때 GitLab CR에서 `nginx-ingress.enabled=false`을(를) 설정하고 OpenShift Routes가 외부 트래픽의 흐름을 제어하도록 할 수 있습니다. GitLab Operator가 Ingress 객체를 조정할 때, OpenShift는 자동으로 클러스터의 기본 도메인과 매핑되는 동등한 Route 객체를 생성합니다.

OpenShift Routes는 TCP 트래픽(포트 `22`의 SSH)을 노출하는 것을 지원하지 않으므로, `gitlab-shell`를 통한 Git over SSH에는 사용할 수 없습니다.

## 고려 사항 {#considerations}

다음은 Ingress를 사용할 때 고려해야 할 항목들입니다.

### OpenShift에서 제3자 Ingress 제공자 사용 {#using-a-third-party-ingress-provider-in-openshift}

OpenShift에서 제3자 Ingress 컨트롤러를 사용할 때, OpenShift Ingress 컨트롤러는 경우에 따라 제3자 Ingress 컨트롤러와 충돌할 수 있습니다.

한 가지 예는 NGINX Ingress 컨트롤러가 Ingress `ADDRESS`을(를) NGINX Service의 외부 IP 주소로 설정하고, 그러면 OpenShift Ingress 컨트롤러가 이를 클러스터의 기본 도메인으로 재정의한다는 것입니다. 이는 DNS 구성과 충돌할 수 있으며, 특히 [external-dns](https://github.com/kubernetes-sigs/external-dns)와 같은 서비스를 사용할 때 문제가 됩니다. 이 서비스는 Ingress가 IP 주소를 가지고 있어야 하므로 URL을 특정 NGINX Service로 매핑하기 위한 A 레코드를 생성할 수 있습니다. 이는 GitLab Operator CI 환경 내에서의 경우입니다.

이를 해결하기 위해 [OpenShift Ingress 컨트롤러를 패치](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/558e2ff9/ci/scripts/install_external_dns.sh#L17-26)하여 OpenShift 관련 네임스페이스만 관리하도록 하고, GitLab 관련 네임스페이스에서 만든 Ingress가 원치 않게 수정되지 않도록 합니다.
