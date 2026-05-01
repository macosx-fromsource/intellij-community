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

[GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator)는 [Kubernetes Operator 패턴](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)을 따르는 설치 및 관리 방법입니다.

GitLab Operator를 사용하여 [OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/) 또는 다른 Kubernetes 호환 플랫폼에서 GitLab을 실행하세요.

> [!note]
> GitLab Operator에는 [알려진 제한 사항](#known-issues)이 있으며, 프로덕션 환경에서는 특정 시나리오에만 적합합니다.

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

> [!warning]
> GitLab 커스텀 리소스의 기본값은 **프로덕션 환경에서 사용하도록 설계되지 않았습니다**.
> 이 기본값을 사용하면 GitLab Operator는 영구 데이터를 포함한 모든 서비스가 Kubernetes 클러스터에 배포되는 GitLab 인스턴스를 생성하며, 이는 **프로덕션 워크로드에 적합하지 않습니다**.
> 프로덕션 배포의 경우 [Cloud Native Hybrid 참조 아키텍처](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid)를 **반드시** 따라야 합니다.
> GitLab은 Kubernetes 클러스터 내부에 배포된 PostgreSQL, Redis, Gitaly, Praefect 또는 MinIO와 관련된 문제를 지원하지 않습니다.

## 알려진 문제 {#known-issues}

GitLab Operator는 다음을 지원하지 않습니다.

- GitLab Operator로 기존 Helm 차트 기반 인스턴스 관리. 개선 지원은 [GitLab Operator issue 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567)에서 제안되었습니다.
- [OpenShift 라우트](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html)를 통한 SSH 방식의 Git 사용. 자세한 내용은 [OpenShift 라우트에 관한 GitLab Operator 문서](openshift_ingress.md#openshift-routes)를 참조하세요.
- 다른 클라우드 API(예: 오브젝트 스토리지)에 대한 워크로드 인증을 위한 [GKE 워크로드 신원](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) 및 [IAM 서비스 계정](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html). 자세한 내용은 [GitLab Operator issue 1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737)를 참조하세요.
- 기본적으로 Operator는 제로 다운타임 방식으로 GitLab을 업그레이드합니다. 따라서 GitLab 및 GitLab 차트 버전은 마이너 릴리즈 단위로 업데이트해야 합니다. 마이너 버전을 건너뛰려면 [제로 다운타임 업그레이드를 비활성화](gitlab_upgrades.md#upgrade-with-downtime)할 수 있으며, 이 경우 업그레이드 중 다운타임이 발생합니다.

GitLab Operator는 GitLab 차트의 다른 모든 제한 사항도 가집니다. GitLab Operator는 Kubernetes 자원을 프로비저닝하기 위해 GitLab 차트에 의존합니다. 따라서 GitLab 차트의 모든 제한 사항은 GitLab Operator에도 영향을 미칩니다. GitLab Operator에서 GitLab 차트 의존성을 제거하는 방안은 [Cloud Native epic 64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64)에서 제안되었습니다.

## 설치 {#installation}

GitLab Operator 설치 방법은 [설치 문서](installation.md)에서 확인할 수 있습니다.

[Security Context Constraints](security_context_constraints.md) 사용 방법에 대한 자세한 내용은 해당 문서를 참조하세요.

또한 [Git에 대한 SSH 액세스 고려 사항](git_over_ssh.md)도 숙지해야 하며, 특히 OpenShift를 사용할 때 중요합니다.

## 업그레이드 {#upgrading}

GitLab Operator 또는 GitLab Operator가 관리하는 GitLab 인스턴스를 업그레이드하는 방법은 [GitLab Operator로 GitLab 인스턴스 업그레이드](gitlab_upgrades.md)를 참조하세요.

## 백업 및 복원 {#backup-and-restore}

[백업 및 복원](backup_and_restore.md) 문서에서는 Operator가 관리하는 GitLab 인스턴스를 백업하고 복원하는 방법을 설명합니다.

## RedHat 인증 이미지 사용 {#using-redhat-certified-images}

[RedHat 인증 이미지](certified_images.md) 문서에서는 GitLab Operator가 RedHat에서 인증한 이미지를 배포하도록 설정하는 방법을 설명합니다.

## 개발자 도구 {#developer-tooling}

- [개발자 가이드](developer/guide.md): 프로젝트 구조와 기여 방법을 설명합니다.
- [버전 관리 및 릴리즈 정보](developer/releases.md): 버전 관리 및 Operator 릴리즈에 관한 노트를 기록합니다.
- [설계 결정 사항](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr): 이 프로젝트는 아키텍처 결정 기록(ADR)을 활용하여 Operator의 구조, 기능 및 기능 구현을 상세히 설명합니다.

## 머지 리퀘스트 리뷰 {#merge-request-reviews}

머지 리퀘스트(MR)는 일반적으로 2명의 검토자가 필요한 표준 절차를 따릅니다. 먼저 유지관리자가 아닌 검토자가 MR을 검토하고 제안된 변경 사항을 개선하거나 수정하는 데 도움이 되는 댓글을 작성자에게 제공합니다. 작성자가 필요한 수정을 완료하고 검토자가 MR을 승인하면, 유지관리자 중 한 명에게 리뷰를 요청합니다.

이 방식은 경험이 적은 검토자에게 학습 기회를 제공합니다. 첫 번째 리뷰에서 최종 리뷰 전에 MR의 대부분의 문제를 해결합니다. 대용량 프로젝트는 유지관리자 부하로 인해 병목 현상이 발생하는 경우가 많으며, 이 첫 번째 검토 단계가 유지관리자의 부담을 줄이는 데 도움이 됩니다.

### 단일 승인 예외 {#one-approval-only-exceptions}

특정 경우에는 MR을 단일 승인만으로 머지할 수 있습니다.

#### Go 모듈 업데이트 {#go-modules-updates}

> [!note]
> 이 내용은 이 프로젝트를 소유한 그룹의 GitLab 팀 멤버에게만 해당됩니다.

이 프로젝트를 소유한 팀의 멤버라면 `go.mod` 및 `go.sum` 파일에 대한 CODEOWNERS 승인 권한이 부여됩니다. MR이 이 파일들만 변경하는 경우, 유지관리자가 아니더라도 MR을 승인하고 머지할 수 있습니다. 이는 Go 모듈 업데이트가 매우 낮은 위험도를 가진다는 팀의 평가를 바탕으로 유지관리자의 리뷰 부담을 줄이고 의존성 업데이트 효율성을 높이기 위해 도입되었습니다. 따라서 Go에 익숙하고, 변경 사항이 적절해 보이며, MR에서 파이프라인이 완전히 통과된 경우 바로 승인하고 머지하세요.

단, Go 코드에 익숙하지 않거나 다른 이유로 추가 의견이 필요한 경우, MR을 유지관리자에게 전달하여 두 번째 리뷰를 받는 것도 권장됩니다.
