---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 설치
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

> [!note]
> GitLab Operator에는 [알려진 제한 사항](_index.md#known-issues)이 있으며, 프로덕션 환경에서는 특정 시나리오에만 적합합니다.

<!--This warning block is duplicated in doc/index.md. Changes should be reflected in both locations.-->

> [!warning]
> GitLab 커스텀 리소스의 기본값은 **프로덕션 환경에서 사용하도록 설계되지 않았습니다**.
> 이 값을 사용하면 GitLab Operator는 영구 데이터를 포함한 모든 서비스가
> Kubernetes 클러스터에 배포되는 GitLab 인스턴스를 생성하며, 이는 **프로덕션 워크로드에 적합하지 않습니다**.
> 프로덕션 배포의 경우 [Cloud Native Hybrid 참조 아키텍처](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)를 **반드시** 따라야 합니다.
> GitLab은 Kubernetes 클러스터 내부에 배포된 PostgreSQL, Redis, Gitaly, Praefect 또는 MinIO와 관련된 문제를 지원하지 않습니다.

이 문서에서는 Kubernetes 또는 OpenShift 클러스터에서 매니페스트를 사용하여 GitLab Operator를 배포하는 방법을 설명합니다.

<!--This warning block is duplicated in `../config/manifests/bases/gitlab-operator-kubernetes.clusterserviceversion.yaml`.
Changes should be reflected in both locations.-->

OpenShift를 사용하는 경우, 설치는 일반적으로 OLM(Operator Lifecycle Manager)에 의해 처리됩니다.
**OLM을 사용한 설치는 실험적인 것으로 간주됩니다**. GitLab은 OLM을 사용하여 배포된 인스턴스와 관련된 문제를 지원하지 않습니다.
OLM의 잠재적인 문제에 대한 자세한 내용은 [이슈 241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)을 참조하세요.

## 사전 요구 사항 {#prerequisites}

1. [Kubernetes 또는 OpenShift 클러스터 생성 또는 기존 클러스터 사용](#cluster)
1. 사전 요구 서비스 및 소프트웨어 설치
   - [Ingress 컨트롤러](#ingress-controller)
   - [cert-manager](#tls-certificates)
   - [메트릭 서버](#metrics)
1. [도메인 네임 서비스 구성](#configure-domain-name-services)

### 클러스터 {#cluster}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

기존 Kubernetes 클러스터를 생성하려면 [공식 도구](https://kubernetes.io/docs/tasks/tools/) 또는 원하는 설치 방법을 사용하세요.

GitLab Operator는 다음 Kubernetes 버전을 지원합니다:

| Kubernetes 릴리즈 | 상태        | 최소 Operator 버전 |
|--------------------|-------------|--------------------------|
| 1.35               | 지원됨      | 2.9.0                    |
| 1.34               | 지원됨      | 2.5.0                    |
| 1.33               | 지원됨      | 2.1.0                    |
| 1.32               | 지원 중단   | 2.0.0                    |
| 1.31               | 미지원      | 1.9.0                    |

{{< /tab >}}

{{< tab title="OpenShift" >}}

GitLab Operator는 다음 OpenShift 버전을 지원합니다:

| OpenShift 릴리즈 | 상태        | 최소 Operator 버전 |
|-------------------|-------------|--------------------------|
| 4.21              | 지원됨      | 2.9.0                    |
| 4.20              | 지원됨      | 2.6.0                    |
| 4.19              | 지원됨      | 2.2.0                    |
| 4.18              | 지원됨      | 1.9.0                    |
| 4.17              | 미지원      | 1.6.0                    |

{{< /tab >}}

{{< /tabs >}}

Kubernetes의 최근 3개 마이너 버전과 OpenShift의 최근 4개 마이너 릴리즈와의 호환성을 동시에 목표로 합니다. 새 버전에 대한 지원이 추가되면 가장 오래된 지원 버전에 대한 테스트는 중단됩니다. Kubernetes 및 OpenShift의 새 마이너 릴리즈에 대해 최초 출시 후 3개월 이내에 Operator 지원을 제공하는 것이 목표입니다.

자세한 내용은 [Kubernetes 지원 정책](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)을 참조하세요.

> [!note]
> [Kubernetes용 에이전트](https://docs.gitlab.com/user/clusters/agent/) 및
> [GitLab 차트](https://docs.gitlab.com/charts/installation/cloud/)와 같은 일부 컴포넌트의 경우, GitLab은 다른 클러스터 버전을 지원할 수 있습니다.

위에 나열된 버전보다 최신 릴리즈와의 호환성 문제는 [이슈 트래커](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)에서 환영합니다.

일부 GitLab 기능은 지원 중단된 버전 및 위에 나열된 버전보다 오래된 버전에서는 작동하지 않을 수 있습니다.

Operator는 x86-64 및 ARM64를 지원합니다. ARM64 빌드는 16.7부터 제공되었지만, 전체 지원 및 테스트 커버리지는 18.8부터 제공됩니다.

### Ingress 컨트롤러 {#ingress-controller}

애플리케이션에 대한 외부 액세스와 컴포넌트 간의 보안 통신을 제공하려면 Ingress 컨트롤러가 필요합니다.

GitLab Operator는 기본적으로 [GitLab Helm Chart의 포크된 NGINX 차트](https://docs.gitlab.com/charts/charts/nginx/)를 배포합니다.

외부 Ingress 컨트롤러를 사용하려면 Kubernetes 커뮤니티의 [NGINX Ingress](https://kubernetes.github.io/ingress-nginx/deploy/)를 사용하여 Ingress 컨트롤러를 배포하세요. 플랫폼 및 원하는 도구에 따라 링크의 관련 지침을 따르세요. 나중에 사용할 Ingress 클래스 값을 메모해 두세요(일반적으로 기본값은 `nginx`입니다).
GitLab CR을 구성할 때 GitLab Helm Chart의 NGINX 오브젝트를 비활성화하려면 `nginx-ingress.enabled=false`를 설정해야 합니다.

### TLS 인증서 {#tls-certificates}

Operator의 Kubernetes 웹훅에 대한 인증서를 생성하기 위해 [cert-manager](https://cert-manager.io)가 사용됩니다. GitLab 인증서에도 [cert-manager](https://cert-manager.io)를 사용하는 것이 좋습니다.

Operator는 Kubernetes 웹훅에 대한 인증서가 필요하므로 GitLab Chart에 번들된 cert-manager를 사용할 수 없습니다. 대신 Operator를 설치하기 전에 cert-manager를 설치하세요.

플랫폼 및 도구에 맞는 [지원되는 cert-manager 릴리즈](https://cert-manager.io/docs/releases/)를 설치하려면 [설치 문서](https://cert-manager.io/docs/installation/)를 따르세요.

### 메트릭 {#metrics}

{{< tabs >}}

{{< tab title="Kubernetes" >}}

HorizontalPodAutoscaler가 pod 메트릭을 검색할 수 있도록 [메트릭 서버](https://github.com/kubernetes-sigs/metrics-server#installation)를 설치하세요.

{{< /tab >}}

{{< tab title="OpenShift" >}}

OpenShift는 기본적으로 [Prometheus Adapter](https://docs.openshift.com/container-platform/4.9/monitoring/monitoring-overview.html)와 함께 제공되므로, GitLab Operator가 다른 인스턴스를 설치하지 않도록 GitLab 커스텀 리소스에서 `spec.chart.values.prometheus.install=false`를 설정하기만 하면 됩니다.

{{< /tab >}}

{{< /tabs >}}

### 도메인 네임 서비스 구성 {#configure-domain-name-services}

DNS 레코드를 추가할 수 있는 인터넷 접근 가능한 도메인이 필요합니다.

도메인을 GitLab 컴포넌트에 연결하는 방법에 대한 자세한 내용은 [네트워킹 및 DNS 문서](https://docs.gitlab.com/charts/installation/tools/#networking-and-dns)를 참조하세요. 이 섹션에서 언급된 구성은 GitLab 커스텀 리소스(CR)를 정의할 때 사용합니다.

OpenShift의 Ingress는 추가적인 고려가 필요합니다. 자세한 내용은 [OpenShift Ingress 관련 참고 사항](openshift_ingress.md)을 참조하세요.

## GitLab Operator 설치 {#installing-the-gitlab-operator}

먼저 설치 방법을 선택하세요.

{{< tabs >}}

{{< tab title="Manifest" >}}

먼저 [Operator 릴리즈 페이지](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)에서 릴리즈 매니페스트를 가져오세요.
대상 플랫폼(Kubernetes 또는 OpenShift)에 맞는 매니페스트를 선택하세요.

다음으로, Operator가 설치될 네임스페이스를 생성하세요.
매니페스트에서 네임스페이스는 기본적으로 `gitlab-system`으로 설정됩니다.
네임스페이스를 변경하려면 매니페스트를 수동으로 업데이트하거나, 이 키와 다른 키를 쉽게 구성할 수 있는 Helm 차트 사용을 고려하세요.

```shell
kubectl create namespace gitlab-system
```

마지막으로 매니페스트를 적용하세요:

```shell
kubectl apply -f gitlab-operator-<platform>.yaml
```

{{< /tab >}}

{{< tab title="Helm Chart" >}}

먼저 GitLab Helm 리포지토리를 추가하고 최신 업데이트를 가져오세요.

```shell
helm repo add gitlab https://charts.gitlab.io
helm repo update
```

그런 다음 GitLab Operator 차트를 설치할 수 있습니다:

```shell
helm install gitlab-operator gitlab/gitlab-operator \
  --create-namespace \
  --namespace gitlab-system
```

사용 가능한 모든 구성 옵션은 [`values.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/values.yaml)을 참조하세요.

{{< /tab >}}

{{< tab title="OLM" >}}

GitLab Operator는 다음 OLM 채널에서 사용할 수 있습니다:

- [OperatorHub.io](https://operatorhub.io/operator/gitlab-operator-kubernetes)
- [OpenShift Community Operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod): OpenShift 및 OKD의 내장 OperatorHub에서 제공
- [Red Hat Ecosystem Catalog](https://catalog.redhat.com/software/container-stacks/detail/5ec3fcb08b6f188e53644c0f)

{{< /tab >}}

{{< /tabs >}}

Operator 배포 상태를 확인하여 설치를 검증하세요:

```shell
kubectl -n gitlab-system get deployment gitlab-controller-manager
```

## GitLab 설치 {#installing-gitlab}

1. GitLab 커스텀 리소스(CR)를 생성합니다.

   `mygitlab.yaml`과 같은 이름으로 새 파일을 생성하세요.

   이 파일에 넣을 내용의 예시는 다음과 같습니다:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   spec:
     chart:
       version: "X.Y.Z" # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/<OPERATOR_VERSION>/CHART_VERSIONS
       values:
         global:
           hosts:
             domain: example.com # use a real domain here
           ingress:
             configureCertmanager: true
         certmanager-issuer:
           email: youremail@example.com # use your real email address here
   ```

   `spec.chart.values` 아래에서 사용할 구성 옵션에 대한 자세한 내용은 [GitLab Helm Chart 문서](https://docs.gitlab.com/charts/charts/)를 참조하세요.

1. 새 GitLab CR을 사용하여 GitLab 인스턴스를 배포합니다.

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   이 명령은 GitLab Operator가 조정할 수 있도록 GitLab CR을 클러스터에 전송합니다. 컨트롤러 pod의 로그를 tail하여 진행 상황을 확인할 수 있습니다:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

   GitLab 리소스를 나열하고 상태를 확인할 수도 있습니다:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS   VERSION
   gitlab   Ready    5.2.4
   ```

   CR이 조정되면(GitLab 리소스의 상태가 `Running`인 경우) 브라우저에서 `https://gitlab.example.com`으로 GitLab에 액세스할 수 있습니다.

로그인하려면 배포의 초기 루트 비밀번호를 검색해야 합니다. 자세한 지침은 [Helm Chart 문서](https://docs.gitlab.com/charts/installation/deployment/#initial-login)를 참조하세요.

## 권장 다음 단계 {#recommended-next-steps}

설치를 완료한 후 인증 옵션 및 가입 제한을 포함한 [권장 다음 단계](https://docs.gitlab.com/install/next_steps/)를 따르는 것을 고려하세요.

### OpenShift {#openshift}

OpenShift를 실행하는 경우 GitLab Operator의 승인 전략을 자동(기본값)에서 수동으로 변경하세요. 이렇게 하면 [승인이 주어질](https://docs.openshift.com/container-platform/4.13/operators/admin/olm-upgrading-operators.html#olm-approving-pending-upgrade_olm-upgrading-operators) 때까지 OpenShift가 새 Operator 버전을 설치하지 않습니다.

커스텀 [`startingCSV`](https://docs.openshift.com/container-platform/4.10/operators/admin/olm-adding-operators-to-cluster.html#olm-installing-specific-version-cli_olm-adding-operators-to-a-cluster)를 설정하여 Operator 버전을 고정하거나 최신이 아닌 버전으로 업그레이드할 수도 있습니다.

- 승인 전략은 [OpenShift 웹 콘솔](https://access.redhat.com/documentation/en-us/red_hat_openshift_data_foundation/4.13/html/updating_openshift_data_foundation/changing-the-update-approval-strategy_rhodf)에서 변경하거나 [Subscription을 편집](https://docs.openshift.com/container-platform/4.13/operators/understanding/olm/olm-understanding-olm.html#olm-installplan_olm-understanding-olm)하여 변경할 수 있습니다.
- 수동 업그레이드를 승인하려면 `InstallPlan`의 `.spec.approved`를 `true`로 설정하세요.
- 각 GitLab Operator는 정의된 GitLab 차트 버전의 하위 집합을 지원합니다. GitLab Operator 업그레이드 시 GitLab 커스텀 리소스의 차트 버전도 함께 업데이트해야 합니다.
- GitLab Operator와 지정된 GitLab Helm 차트 버전이 호환되지 않는 경우, 차트에 대한 구성 변경이 [GitLab Helm 차트 버전 관련 오류](gitlab_upgrades.md)와 함께 실패할 수 있습니다.

> [!note]
> [OLM은 Operator 다운그레이드를 지원하지 않습니다](https://github.com/operator-framework/operator-lifecycle-manager/issues/1177).

## GitLab Operator 제거 {#uninstall-the-gitlab-operator}

GitLab Operator 및 관련 리소스를 제거하려면 아래 단계를 따르세요.

Operator를 제거하기 전에 유의할 사항:

- Operator는 GitLab 인스턴스가 삭제될 때 Persistent Volume Claims 또는 Secrets를 삭제하지 않습니다.
- Operator를 삭제할 때 설치된 네임스페이스(`gitlab-system` 기본값)는 자동으로 삭제되지 않습니다. 이는 영구 볼륨이 의도치 않게 손실되지 않도록 보장합니다.

### GitLab 인스턴스 제거 {#uninstall-an-instance-of-gitlab}

```shell
kubectl -n gitlab-system delete -f mygitlab.yaml
```

이 명령은 GitLab 인스턴스와 위에서 언급한 Persistent Volume Claims를 제외한 모든 관련 오브젝트를 제거합니다.

### GitLab Operator 제거 {#uninstall-the-gitlab-operator}

```shell
GL_OPERATOR_VERSION=<your_installed_version> # https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases
PLATFORM=kubernetes # or "openshift"
kubectl delete -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${GL_OPERATOR_VERSION}/gitlab-operator-${PLATFORM}-${GL_OPERATOR_VERSION}.yaml
```

이 명령은 실행 중인 Operator 배포를 포함하여 Operator의 리소스를 삭제합니다. GitLab 인스턴스와 관련된 오브젝트는 **삭제되지 않습니다**.

## GitLab Operator 문제 해결 {#troubleshoot-the-gitlab-operator}

GitLab Operator 문제 해결에 대한 자세한 내용은 [문제 해결](troubleshooting.md)을 참조하세요.
