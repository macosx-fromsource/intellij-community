---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Operator로 GitLab 인스턴스 업그레이드
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

GitLab Operator를 사용하여 GitLab Operator로 설치된 GitLab 인스턴스를 업그레이드할 수 있습니다.

## 사전 요구 사항 {#prerequisites}

GitLab Operator로 업그레이드하기 전에:

1. [업그레이드 전에 필요한 정보](https://docs.gitlab.com/update/plan_your_upgrade/)를 확인하세요.
1. 원하는 GitLab 버전에 필요한 GitLab Operator 버전을 확인하세요. GitLab 버전, GitLab Helm 차트 버전, GitLab Operator 버전 간의 매핑은 GitLab Operator
   [릴리즈](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)를 참조하세요.

## GitLab Operator로 GitLab 업그레이드 {#upgrade-gitlab-with-gitlab-operator}

GitLab Operator로 GitLab을 업그레이드하려면:

1. 업그레이드 중 사용자의 쓰기 작업을 제한하여 워크플로우 방해를 최소화하려면 [유지 관리 모드 활성화](https://docs.gitlab.com/administration/maintenance_mode/)를 고려하세요.
1. [GitLab Runner 업그레이드](https://docs.gitlab.com/runner/install/)를 대상 GitLab 버전과 동일한 버전으로 진행하세요.
1. [GitLab Operator 업그레이드](#upgrade-gitlab-operator)를 진행하세요.
1. [GitLab Operator를 사용하여 GitLab 업그레이드](#upgrade-gitlab-by-using-gitlab-operator)를 진행하세요.

업그레이드 후:

1. 활성화된 경우 [유지 관리 모드를 비활성화](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode)하세요.
1. [업그레이드 헬스 체크](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks)를 실행하세요.

### GitLab Operator 업그레이드 {#upgrade-gitlab-operator}

GitLab Operator를 업그레이드하려면:

1. [백업](https://docs.gitlab.com/charts/backup-restore/)을 수행하세요.
1. `kubectl`을 사용하여 필요한 GitLab Operator 버전의 매니페스트를 적용하여 필요한 버전을 설치하세요.

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   이 명령은 새 배포 이미지를 포함하여 관련 매니페스트의 모든 변경 사항을 적용합니다.

1. 새 버전의 GitLab Operator가 리더가 되는지 확인하세요. GitLab Operator 배포는 이 변경으로 새 ReplicaSet을 생성하고, 새 GitLab Operator 파드를 생성합니다. 이때 이전 GitLab Operator 파드는 리더 상태를 포기하고 종료됩니다. 이 과정이 완료되면 새 GitLab Operator 파드가 리더가 됩니다.
1. GitLab 커스텀 리소스(CR)의 차트 버전을 업데이트하세요. 대부분의 경우 GitLab Operator 버전 간에 사용 가능한 차트 버전이 동일하지 않습니다. 새 버전의 GitLab Operator가 시작되면 기존 GitLab 커스텀 리소스(CR)를 조정하려고 시도합니다. 다음과 같은 오류가 발생할 수 있습니다:

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   이를 해결하려면 해당 릴리즈의 사용 가능한 차트 버전에서 유효한 버전을 확인하세요. 예를 들어 Operator `0.4.0`에서 `0.4.1`로 업그레이드할 때, GitLab CR을 `5.7.0`에 가장 가까운 사용 가능한 차트 버전(이 경우 `5.7.1`)으로 업데이트하세요.

1. GitLab Operator가 예상대로 GitLab을 조정하는지 확인하세요. 새 Operator 파드의 로그를 확인하여 Operator 파드가 정의된 차트 버전으로 업그레이드되었는지 확인하세요.

   업그레이드가 성공했는지 확인하려면 GitLab CR의 상태를 확인하세요:

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   `Running` 상태는 GitLab Operator가 인스턴스의 변경 사항을 성공적으로 조정했음을 의미합니다. 버전은 GitLab Operator 업그레이드 후 지정된 차트 버전과 일치해야 합니다.

#### 문제 해결 {#troubleshooting}

오류가 발생하면 먼저
[문제 해결 문서](troubleshooting.md)를 참조하세요.
해당 문서에서 답을 찾을 수 없는 경우 기존 이슈를 확인하거나
[이슈 트래커](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)에서 새 이슈를 등록하세요.

### GitLab Operator를 사용하여 GitLab 업그레이드 {#upgrade-gitlab-by-using-gitlab-operator}

> [!warning]
> 기본적으로 GitLab Operator는 [제로 다운타임](https://docs.gitlab.com/update/zero_downtime/) 방식으로 GitLab을 업그레이드합니다.
> 따라서 GitLab 및 기반 차트 버전은 한 번에 하나의 마이너 릴리즈씩 업데이트해야 합니다.
>
> 2.6.0 및 2.5.1 이전 Operator 릴리즈는 유효한 업그레이드 경로를 강제하지 않았습니다.
>
> 업그레이드 중 마이너 버전을 건너뛰려면 [다운타임 업그레이드](#upgrade-with-downtime)를 사용해야 합니다.

1. GitLab 커스텀 리소스의 `spec.chart.version` 필드를 새 버전으로 업데이트하세요. 예를 들어:

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   spec:
     chart:
   -   version: "5.0.6"
   +   version: "5.1.1"
       values:
         ...
   ```

1. 수정된 GitLab 커스텀 리소스를 클러스터에 적용하세요:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   다음 메시지가 표시됩니다:

   ```shell
   gitlab.apps.gitlab.com/gitlab created
   ```

   컨트롤러 로그에서 진행 상황을 확인할 수 있습니다. 예를 들어:

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   위에 설명된 업그레이드 단계에 따른 로그 항목이 표시됩니다. 클러스터에서 GitLab 커스텀 리소스 상태도 확인할 수 있습니다:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS        VERSION
   gitlab   Preparing     5.2.4
   ```

애플리케이션이 준비되어 새 버전으로 업그레이드되면 `STATUS` 열에 반영됩니다.

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

GitLab 오브젝트의 상태 조건에서 애플리케이션에 대한 더 자세한 정보를 확인할 수 있습니다.

### 다운타임 업그레이드 {#upgrade-with-downtime}

기본적으로 GitLab Operator는 [제로 다운타임 업그레이드](https://docs.gitlab.com/update/zero_downtime/) 경로를 강제하며,
한 번에 하나의 마이너 버전씩 업데이트해야 합니다. 마이너 버전을 건너뛰려는 경우(예: GitLab 18.0에서 18.2로 업그레이드),
GitLab 커스텀 리소스에 `gitlab.io/disable-zero-downtime-upgrade` 어노테이션을 추가하여 제로 다운타임 업그레이드를 비활성화할 수 있습니다.

> [!warning]
> 다운타임 업그레이드 중에는 데이터베이스 마이그레이션이 실행되는 동안 GitLab 인스턴스를 사용할 수 없습니다.
> 진행하기 전에 유지 관리 기간을 계획하고 예상 다운타임을 사용자에게 공지했는지 확인하세요.
>
> 마이너 버전을 건너뛸 때도 필수 [업그레이드 중단점](https://docs.gitlab.com/update/upgrade_paths/)을 따라야 합니다. 대상 버전으로 진행하기 전에 각 필수 버전에서 반드시 중단하도록 업그레이드 경로를 계획하세요.

다운타임 업그레이드를 수행하려면:

1. [유지 관리 모드 활성화](https://docs.gitlab.com/administration/maintenance_mode/)를 고려하세요.
1. [백업](https://docs.gitlab.com/charts/backup-restore/)을 수행하세요.
1. 대상 GitLab 버전을 지원하는 버전으로 GitLab Operator를 업그레이드하세요.
1. GitLab 커스텀 리소스에 `gitlab.io/disable-zero-downtime-upgrade` 어노테이션을 추가하고 `spec.chart.version`을 업데이트하세요:

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   + annotations:
   +   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
   -   version: "9.0.0"
   +   version: "9.2.0"
       values:
         ...
   ```

1. 수정된 GitLab 커스텀 리소스를 적용하세요:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   Operator가 자동으로 다음을 수행합니다:

   1. Webservice 및 Sidekiq 배포를 복제품 0개로 스케일 다운합니다.
   1. 모든 파드가 종료될 때까지 대기합니다.
   1. 데이터베이스 마이그레이션을 실행합니다.
   1. 새 차트 버전으로 Webservice 및 Sidekiq를 조정합니다.
   1. 차트 값에서 Webservice 및 Sidekiq 복제품 수를 복원합니다.

1. 업그레이드 진행 상황을 모니터링하세요:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

1. 업그레이드가 완료될 때까지 기다리세요:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS      VERSION
   gitlab   Running     9.3.0
   ```

1. 업그레이드가 완료된 후 향후 업그레이드에서 기본 제로 다운타임 업그레이드 동작을 복원하려면 어노테이션을 제거하세요:

   ```diff
   apiVersion: apps.gitlab.com/v1beta1
   kind: GitLab
   metadata:
     name: gitlab
   - annotations:
   -   gitlab.io/disable-zero-downtime-upgrade: "true"
   spec:
     chart:
       version: "9.3.0"
       values:
         ...
   ```

1. 활성화된 경우 [유지 관리 모드를 비활성화](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode)하세요.

## GitLab Operator의 GitLab 업그레이드 방식 {#how-gitlab-operator-upgrades-gitlab}

컨트롤러 조정 루프가 시작될 때 GitLab Operator는 현재 버전이 필요한 버전과 일치하는지 확인합니다.

- 버전이 일치하면 일반 조정 루프가 실행되어 커스텀 리소스(CR) 스펙에 제공된 구성을 충족하는 오브젝트가 존재하는지 확인합니다.
- 버전이 일치하지 않으면 일반 조정 루프는 계속 실행되지만, 업그레이드를 처리하기 위한 추가 로직 분기가 실행됩니다.

업그레이드 과정:

1. 컨트롤러가 모든 배포를 조정합니다. Webservice 및 Sidekiq 배포는 조정되지만 일시 중지됩니다.
   새 배포가 재개될 때까지 이전 파드는 계속 실행됩니다.
1. 사전 마이그레이션이 실행되어 Migrations 작업을 실행하지만 배포 후 마이그레이션은 건너뜁니다.
1. 컨트롤러가 Webservice 및 Sidekiq 배포를 재개합니다.
1. 컨트롤러가 새 Webservice 및 Sidekiq 파드가 실행될 때까지 대기합니다.
1. 사후 마이그레이션이 실행되어 배포 후 마이그레이션을 건너뛰지 않고 Migrations 작업을 실행합니다.
1. 컨트롤러가 Webservice 및 Sidekiq 배포에 대해 롤링 업데이트를 수행합니다.
1. 컨트롤러가 재시작된 Webservice 및 Sidekiq 파드가 실행될 때까지 대기합니다.

이후 조정 루프에서는 원하는 버전(`spec.chart.version`)이 현재 버전(`status.version`)과 일치하므로 이 로직 분기는 건너뜁니다.

## 관련 항목 {#related-topics}

- [Helm 차트 설치 업그레이드](https://docs.gitlab.com/charts/installation/upgrade/)
- [GitLab Helm 차트 버전](https://docs.gitlab.com/charts/installation/version_mappings/)
- [업그레이드 경로 계획](https://docs.gitlab.com/update/upgrade_paths/)
- [GitLab 업그레이드 노트](https://docs.gitlab.com/update/versions/)
- [GitLab 버전 간 변경 사항](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
