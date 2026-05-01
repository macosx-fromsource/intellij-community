---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 백업 및 복원
---

{{< details >}}

- Tier: Free, 프리미엄, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

GitLab Operator는 [Toolbox 차트](https://docs.gitlab.com/charts/charts/gitlab/toolbox/)를 배포합니다.
Toolbox를 사용하여 GitLab 인스턴스를 백업하고 복원하려면 [GitLab 백업 및 복원](https://docs.gitlab.com/charts/backup-restore/)을 참조하세요.

## Helm 기반 설치와 Operator 기반 설치 간 마이그레이션 {#migrate-between-helm-based-and-operator-based-installations}

Operator 기반 인스턴스 백업에서 새 Helm 차트 기반 인스턴스를 생성하거나, Helm 차트 기반 인스턴스 백업에서 새 Operator 기반 인스턴스를 생성할 수 있습니다.

PostgreSQL 및 Gitaly와 같은 스테이트풀 구성 요소에 외부 서비스를 사용하는 환경은 일반적으로 마이그레이션이 더 쉽습니다.
