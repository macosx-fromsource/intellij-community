---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator) 는 [Kubernetes Operator 패턴](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)을 따르는 설치 및 관리 방법입니다.

GitLab Operator를 사용하여 [OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/)에서 또는 다른 Kubernetes 호환 플랫폼에서 GitLab을 실행합니다.

> [!note]
> GitLab Operator에는 [알려진 제한 사항](#known-issues)이 있으며 프로덕션 환경에서 특정 시나리오에만 적합합니다.

GitLab Operator에는 외부 [PostgreSQL](https://docs.gitlab.com/charts/advanced/external-db/), [Redis](https://docs.gitlab.com/charts/advanced/external-redis/), 및 [오브젝트 스토리지](https://docs.gitlab.com/charts/advanced/external-object-storage/)가 필요합니다.

프로덕션 배포의 경우 [Cloud Native 참조 아키텍처](https://docs.gitlab.com/administration/reference_architectures)를 따르세요.

## 알려진 문제 {#known-issues}

GitLab Operator는 다음을 지원하지 않습니다:

- GitLab Operator로 기존 Helm 차트 기반 인스턴스를 관리합니다. 개선 사항을 위한 지원이 [GitLab Operator issue 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)에서 제안됩니다.
- [OpenShift 경로](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)를 사용한 SSH를 통한 Git입니다. 자세한 내용은 [GitLab Operator documentation on OpenShift Routes](openshift_ingress.md#openshift-routes)를 참조하세요.
- [GKE workload identity](https://docs.cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) 및 [IAM service accounts](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html)를 사용하여 다른 클라우드 API(예: 객체 스토리지)에 워크로드를 인증합니다. 자세한 내용은 [GitLab Operator issue 1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)를 참조하세요.
- 기본적으로 Operator는 zero downtime 방법을 사용하여 GitLab을 업그레이드합니다. 그 결과 GitLab 및 GitLab 차트 버전은 한 번에 한 마이너 릴리스씩 업데이트되어야 합니다. 마이너 버전을 건너뛰려면 [zero-downtime 업그레이드를 비활성화](gitlab_upgrades.md#upgrade-with-downtime)할 수 있으며, 이는 업그레이드 중에 다운타임을 발생시킵니다.

GitLab Operator는 GitLab 차트의 다른 제한 사항이 있습니다. GitLab Operator는 Kubernetes 리소스를 프로비전하기 위해 GitLab 차트에 의존합니다. 따라서 GitLab 차트의 모든 제한 사항은 GitLab Operator에 영향을 미칩니다. GitLab Operator에서 GitLab 차트 종속성을 제거하는 것이 [Cloud Native epic 64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)에서 제안됩니다.

## 설치 {#installation}

GitLab Operator를 설치하는 방법에 대한 지침은 [설치 문서](installation.md)에서 찾을 수 있습니다.

[Security Context Constraints](security_context_constraints.md)를 사용하는 방법에 대한 세부 정보가 각각의 문서에 나열되어 있습니다.

특히 OpenShift를 사용할 때 [Git에 대한 SSH 액세스 고려 사항](git_over_ssh.md)에 대해 알고 있어야 합니다.

## 업그레이드 {#upgrading}

GitLab Operator를 업그레이드하거나 GitLab Operator로 관리하는 GitLab 인스턴스를 업그레이드하는 방법은 [GitLab Operator로 GitLab 인스턴스 업그레이드](gitlab_upgrades.md)를 참조하세요.

## 백업 및 복원 {#backup-and-restore}

[백업 및 복원](backup_and_restore.md) 문서는 Operator로 관리하는 GitLab 인스턴스를 백업 및 복원하는 방법을 보여줍니다.

## RedHat 인증 이미지 사용 {#using-redhat-certified-images}

[RedHat 인증 이미지](certified_images.md) 문서는 GitLab Operator에 RedHat에서 인증한 이미지를 배포하도록 지시하는 방법을 보여줍니다.

## Developer Tooling {#developer-tooling}

- [개발자 가이드](developer/guide.md): 프로젝트 구조와 기여하는 방법을 설명합니다.
- [버전 관리 및 릴리스 정보](developer/releases.md): 연산자의 버전 관리 및 릴리스와 관련된 노트를 기록합니다.
- [설계 결정](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/tree/master/doc/developer/adr): 이 프로젝트는 Architecture Decision Records를 사용하여 이 Operator의 구조, 기능 및 기능 구현을 세부적으로 설명합니다.

## 머지 리퀘스트 검토 {#merge-request-reviews}

머지 리퀘스트(MR)는 일반적으로 2명의 검토자를 요구하는 표준 관행을 따릅니다. 먼저 유지보수자가 아닌 사용자가 머지 리퀘스트를 검토하고 제안된 변경 사항을 개선/수정하는 데 도움이 되는 의견을 작성자에게 제공합니다. 작성자가 필요한 업데이트를 수행하고 검토자가 머지 리퀘스트를 승인한 후, 유지보수자 중 한 명에게 검토를 요청합니다.

이 방식은 경험이 적은 검토자에게 학습 기회를 제공합니다. 첫 번째 검토는 최종 검토 이전에 머지 리퀘스트의 대부분의 문제를 해결합니다. 높은 볼륨의 프로젝트는 유지보수자 부하로 인한 병목 현상을 자주 겪으며 이 첫 번째 검토는 부하를 줄이는 데 도움이 됩니다.

### 승인만 필요한 예외 {#one-approval-only-exceptions}

특정 경우에 우리는 하나의 승인만으로 머지 리퀘스트를 병합할 수 있도록 허용합니다.

#### Go 모듈 업데이트 {#go-modules-updates}

> [!note]
> 이는 이 프로젝트를 소유하는 그룹의 GitLab 팀 구성원에게만 해당됩니다.

이 프로젝트를 소유하는 팀의 팀 구성원인 경우 `go.mod` 및 `go.sum` 파일에 대한 코드 소유자 승인 권한이 부여되었습니다. 머지 리퀘스트가 이러한 파일만 변경하는 경우 유지보수자가 아니더라도 머지 리퀘스트를 승인하고 병합할 수 있어야 합니다. 이는 유지보수자의 검토 부하를 줄이고 종속성 업데이트 효율성을 개선하기 위해 구현되었으며, 팀이 go 모듈에 대한 업데이트는 매우 낮은 위험도라고 평가했습니다. 따라서 Go에 편하고 변경 사항이 좋아 보이며 머지 리퀘스트에 완전히 녹색 파이프라인이 있다면 자유롭게 승인하고 즉시 병합하세요.

그래도 Go 코드에 편하지 않거나 다른 이유로 두 번째 의견을 원한다면 머지 리퀘스트를 유지보수자에게 전달하여 두 번째 검토를 수행하도록 권장합니다.
