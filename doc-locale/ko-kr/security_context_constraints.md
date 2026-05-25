---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 보안 컨텍스트 제약
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab Self-Managed

{{< /details >}}

## 개요 {#overview}

OpenShift의 Pod는 보안 컨텍스트 제약을 기반으로 권한을 받습니다. 보안 컨텍스트 제약은 종종 **SCC**로 약칭되며, 대규모 배포에 사용하기 위한 역할 기반 액세스 제어 메커니즘을 간소화합니다. [관리자는 보안 컨텍스트 제약이 작동하는 방식 및 OpenShift에서의 역할에 대해 더 자세히 이해하기 위해 업스트림 설명서를 참조할 수 있습니다](https://docs.openshift.com/container-platform/4.10/authentication/managing-security-context-constraints.html)

관리자는 또한 다음 리소스를 참조할 수 있습니다:

1. [OpenShift에서 보안 컨텍스트 제약 관리](https://www.redhat.com/en/blog/managing-sccs-in-openshift)
1. [OpenShift 및 UID 가이드](https://www.redhat.com/en/blog/a-guide-to-openshift-and-uids)

## GitLab 배포 내 보안 컨텍스트 제약 {#security-context-constraints-within-the-gitlab-deployment}

`gitlab-controller-manager` 배포는 **Operator** 프로세스를 포함하는 Pod를 생성하고 관리합니다. 이것과 이것이 생성하고 관리하는 다른 Pod는 **restricted** 보안 컨텍스트 제약으로 실행됩니다.

**Operator**는 GitLab 애플리케이션에 필요한 모든 리소스를 관리할 수 있는 견고한 권한을 갖춘 ServiceAccount를 사용합니다.

**Operator**는 Cloud Native GitLab을 구성하는 컴포넌트 서비스를 관리합니다. **Operator**에서 지정한 UID를 따르지 않는 Pod를 적극적으로 종료하고 교체합니다. 이 메커니즘은 최소 권한의 원칙을 적용합니다.

### GitLab 애플리케이션 사용자 정의 리소스 정의 {#gitlab-application-custom-resource-definitions}

Operator에서 GitLab 사용자 정의 리소스를 충족하기 위해 배포된 Pod는 **non-root-v2** 보안 컨텍스트 제약을 사용합니다. 타사 연산자 및 리소스의 보안 컨텍스트 제약은 [다음 섹션에서 다룹니다](#third-party-resource-definitions).

`gitlab-app-nonroot` ServiceAccounts는 권한이 부여되지 않으며 **nonroot-v2** 보안 컨텍스트 제약을 GitLab 애플리케이션 Pod에 바인딩하기 위해서만 존재합니다.

OpenShift 보안 모델 내에서 GitLab 애플리케이션의 전체 읽기/쓰기 동작이 검증됨에 따라 보안 컨텍스트 제약이 향후 릴리스에서 강화될 것입니다.

> [!note]
> Linux 패키지 설치에서 Cloud Native GitLab으로 전환하는 관리자는 `sudo`로 수행되는 Linux 패키지 설치 작업이 OpenShift 및 기본 Kubernetes 엔진에 의해 처리된다는 점에 주의해야 합니다. Pod는 개별 서비스이며, Linux 패키지 설치에서 애플리케이션별 사용자로 실행하기 위해 권한을 제거합니다. **Operator**는 [예상 UID로 작동하지 않는 Pod를 종료합니다](#security-context-constraints-within-the-gitlab-deployment).

### 타사 리소스 정의 {#third-party-resource-definitions}

### Ingress 컨트롤러 {#ingress-controller}

GitLab은 Cloud Native GitLab을 배포할 때 `nginx-ingress-controller`을 사용한 배포를 권장하고 테스트합니다. 자신만의 [`nginx-ingress-scc` 보안 컨텍스트 제약](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/deploy/chart/templates/openshift/scc.yaml)을 사용합니다.

대체 Ingress 컨트롤러를 선택하는 경우 보안 컨텍스트 제약에 대해 자세히 알아보려면 관련 설명서를 참조하시기 바랍니다.

### SSL 암호화 {#ssl-encryption}

GitLab Operator는 [`cert-manager`](https://cert-manager.io/docs/releases/)을 사전 조건으로 별도로 설치해야 합니다. GitLab Operator는 `cert-manager` Issuers 및 Certificates를 구성하여 GitLab 애플리케이션 전체에서 TLS를 관리합니다. `cert-manager`은 보안 컨텍스트 제약을 직접 설정하지 않으므로 OpenShift는 기본적으로 **restricted** 보안 컨텍스트 제약을 적용합니다.
