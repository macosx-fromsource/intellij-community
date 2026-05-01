---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 보안 컨텍스트 제약 조건
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

## 개요 {#overview}

OpenShift의 pod는 보안 컨텍스트 제약 조건에 따라 권한을 부여받습니다. 보안 컨텍스트 제약 조건(흔히 **SCC**로 약칭)은 대규모 배포 환경에서 역할 기반 액세스 제어 메커니즘을 간소화합니다. [운영자는 업스트림 문서를 참조하여 보안 컨텍스트 제약 조건의 작동 방식과 OpenShift에서의 역할에 대해 자세히 알아볼 수 있습니다.](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

운영자는 다음 자료도 참조할 수 있습니다.

1. [OpenShift에서 보안 컨텍스트 제약 조건 관리](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [OpenShift와 UID 가이드](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## GitLab 배포 내 보안 컨텍스트 제약 조건 {#security-context-constraints-within-the-gitlab-deployment}

`gitlab-controller-manager` 배포는 **Operator** 프로세스가 포함된 pod를 생성하고 관리합니다. 이 pod와 Operator가 생성 및 관리하는 다른 모든 pod는 **restricted** 보안 컨텍스트 제약 조건으로 실행됩니다.

**Operator**는 GitLab 애플리케이션에 필요한 모든 자원을 관리할 수 있는 강력한 권한을 가진 ServiceAccount를 사용합니다.

**Operator**는 Cloud Native GitLab을 구성하는 컴포넌트 서비스를 관리합니다. **Operator**가 지정한 UID를 따르지 않는 pod는 즉시 종료하고 교체합니다. 이 메커니즘은 최소 권한 원칙을 강제합니다.

### GitLab 애플리케이션 커스텀 리소스 정의 {#gitlab-application-custom-resource-definitions}

Operator가 GitLab 커스텀 리소스를 충족하기 위해 배포하는 pod는 **non-root-v2** 보안 컨텍스트 제약 조건을 사용합니다. 서드파티 Operator 및 리소스에 대한 보안 컨텍스트 제약 조건은 [다음 섹션에서 다룹니다](#third-party-resource-definitions).

`gitlab-app-nonroot` ServiceAccount에는 부여된 권한이 없으며, GitLab 애플리케이션 pod에 **nonroot-v2** 보안 컨텍스트 제약 조건을 바인딩하는 용도로만 사용됩니다.

보안 컨텍스트 제약 조건은 GitLab 애플리케이션의 전체 읽기/쓰기 동작이 OpenShift 보안 모델 내에서 검증됨에 따라 향후 릴리즈에서 더욱 강화될 예정입니다.

> [!note]
> Linux 패키지 설치 환경에서 Cloud Native GitLab으로 전환하는 운영자는 다음 사항에 유의하세요. Linux 패키지 설치 시 `sudo`로 수행하던 작업은 OpenShift와 기반 Kubernetes 엔진이 처리합니다. Linux 패키지 설치에서는 각 pod가 개별 서비스로서 애플리케이션 전용 사용자로 권한을 낮춰 실행됩니다. **Operator**는 [예상 UID로 동작하지 않는 pod를 종료합니다](#security-context-constraints-within-the-gitlab-deployment).

### 서드파티 리소스 정의 {#third-party-resource-definitions}

### Ingress 컨트롤러 {#ingress-controller}

GitLab은 Cloud Native GitLab 배포 시 `nginx-ingress-controller`를 사용하도록 권장하며 이를 기준으로 테스트합니다. 이 컨트롤러는 자체 [`nginx-ingress-scc` 보안 컨텍스트 제약 조건](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml)을 사용합니다.

다른 Ingress 컨트롤러를 선택하는 경우, 해당 문서를 참조하여 보안 컨텍스트 제약 조건에 대해 자세히 알아보세요.

### SSL 암호화 {#ssl-encryption}

GitLab Operator를 사용하려면 사전에 [`cert-manager`](https://cert-manager.io/docs/releases/)를 별도로 설치해야 합니다. GitLab Operator는 `cert-manager` Issuer와 Certificate를 구성하여 GitLab 애플리케이션 전반의 TLS를 관리합니다. `cert-manager`는 보안 컨텍스트 제약 조건을 직접 설정하지 않으므로, OpenShift는 기본적으로 **restricted** 보안 컨텍스트 제약 조건을 적용합니다.
