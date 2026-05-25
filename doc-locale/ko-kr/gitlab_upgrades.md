---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Operator를 사용하여 GitLab 인스턴스 업그레이드
---

{{< details >}}

- Tier:  Free, Premium, Ultimate
- Offering:  GitLab Self-Managed

{{< /details >}}

GitLab Operator를 사용하여 GitLab Operator로 설치된 GitLab 인스턴스를 업그레이드할 수 있습니다.

## 전제 조건 {#prerequisites}

GitLab Operator로 업그레이드하기 전에:

1. [업그레이드 전에 필요한 정보](https://docs.gitlab.com/update/plan_your_upgrade/)를 확인하세요.
1. 원하는 GitLab 버전에 필요한 GitLab Operator 버전을 확인합니다. GitLab 버전, GitLab Helm 차트 버전, GitLab Operator 버전 간의 매핑은 GitLab Operator [릴리스](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)를 참조하세요.

## GitLab Operator를 사용하여 GitLab 업그레이드 {#upgrade-gitlab-with-gitlab-operator}

GitLab Operator를 사용하여 GitLab을 업그레이드하려면:

1. 업그레이드 중에 [유지 관리 모드 켜기](https://docs.gitlab.com/administration/maintenance_mode/)를 고려하여 사용자의 쓰기 작업을 제한하고 워크플로우를 방해하지 않도록 합니다.
1. [GitLab Runner 업그레이드](https://docs.gitlab.com/runner/install/)를 대상 GitLab 버전과 동일한 버전으로 수행합니다.
1. [GitLab Operator 업그레이드](#upgrade-gitlab-operator)
1. [GitLab Operator를 사용하여 GitLab 업그레이드](#upgrade-gitlab-by-using-gitlab-operator)

업그레이드 후:

1. 활성화된 경우 [유지 관리 모드 끄기](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode)
1. [업그레이드 상태 확인](https://docs.gitlab.com/update/plan_your_upgrade/#run-upgrade-health-checks)을 실행합니다.

### GitLab Operator 업그레이드 {#upgrade-gitlab-operator}

GitLab Operator를 업그레이드하려면:

1. [백업](https://docs.gitlab.com/charts/backup-restore/)을 수행합니다.
1. `kubectl`를 사용하여 GitLab Operator의 필수 버전에 대한 매니페스트를 적용하여 필수 버전을 설치합니다.

   ```shell
   VERSION=X.Y.Z
   kubectl apply -f \
     https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/${VERSION}/gitlab-operator-kubernetes-${VERSION}.yaml
   ```

   이 명령은 사용할 새 배포 이미지를 포함하여 관련 매니페스트에 대한 모든 변경 사항을 적용합니다.

1. GitLab Operator의 새 버전이 리더가 되었는지 확인합니다. GitLab Operator 배포는 이 변경으로 새 ReplicaSet을 생성하여 새 GitLab Operator pod를 생성합니다. 한편, 이전 GitLab Operator pod는 리더 상태를 포기하고 종료됩니다. 이 경우 새 GitLab Operator pod가 리더가 됩니다.
1. GitLab 사용자 정의 리소스(CR)의 차트 버전을 업데이트합니다. 대부분의 경우 GitLab Operator 버전 간에 사용 가능한 차트 버전이 동일하지 않습니다. GitLab Operator의 새 버전이 시작되면 기존 GitLab 사용자 정의 리소스(CR)를 조정하려고 합니다. 다음과 같은 오류가 표시될 수 있습니다:

   ```plaintext
   Configuration error detected: chart version 5.7.0 not supported; please use one of the following: 5.7.1, 5.6.4, 5.5.4
   ```

   이를 해결하려면 해당 릴리스의 사용 가능한 차트 버전에서 유효한 버전을 확인합니다. 예를 들어 Operator `0.4.0`에서 `0.4.1`로 업그레이드할 때 GitLab CR을 `5.7.0`에 가장 가까운 사용 가능한 차트 버전으로 업데이트합니다. 이 경우 `5.7.1`입니다.

1. GitLab Operator가 예상대로 GitLab을 조정하는지 확인합니다. 새 연산자 pod의 로그를 확인하여 연산자 pod가 정의된 차트 버전으로 업그레이드되었는지 확인합니다.

   업그레이드가 성공했는지 확인하려면 GitLab CR의 상태를 가져옵니다:

   ```plaintext
   $ kubectl get gitlabs -n gitlab-system
   NAME     STATUS    VERSION
   gitlab   Running   5.7.1
   ```

   상태 `Running`은 GitLab Operator가 인스턴스에 대한 변경 사항을 조정할 수 있음을 의미합니다. 버전은 GitLab Operator 업그레이드 후 지정된 차트 버전과 일치해야 합니다.

#### 문제 해결 {#troubleshooting}

오류가 발생하면 먼저 [문제 해결 설명서](troubleshooting.md)를 참조하세요. 답변이 제공되지 않은 경우 기존 이슈를 확인하거나 [이슈 추적기](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues)에서 새 이슈를 열어주세요.

### GitLab Operator를 사용하여 GitLab 업그레이드 {#upgrade-gitlab-by-using-gitlab-operator}

> [!warning]
> 기본적으로 GitLab Operator는 [무중단](https://docs.gitlab.com/update/zero_downtime/) 방식을 사용하여 GitLab을 업그레이드합니다. 결과적으로 GitLab 및 기본 차트 버전은 한 번에 하나의 부 릴리스씩 업데이트되어야 합니다.
>
> 2.6.0 및 2.5.1 이전의 Operator 릴리스는 유효한 업그레이드 경로를 적용하지 않았습니다.
>
> 업그레이드 중에 부 버전을 건너뛰려면 [다운타임과 함께 업그레이드](#upgrade-with-downtime)해야 합니다.

1. GitLab 사용자 정의 리소스의 `spec.chart.version` 필드를 새 버전으로 업데이트합니다. 예를 들어:

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

1. 수정된 GitLab 사용자 정의 리소스를 클러스터에 적용합니다:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   다음 메시지가 표시됩니다:

   ```shell
   gitlab.apps.gitlab.com/gitlab created
   ```

   컨트롤러 로그에서 진행 상황을 볼 수 있습니다. 예를 들어:

   ```shell
   $ kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   2021-09-14T20:59:12.342Z        INFO    controllers.GitLab      Reconciling GitLab    {"gitlab": "gitlab-system/gitlab"}
   2021-09-14T20:59:12.344Z        DEBUG   controllers.GitLab      version information   {"gitlab": "gitlab-system/gitlab", "upgrade": true, "current version": "", "desired version": "5.0.6"}
   2021-09-14T20:59:18.168Z        INFO    controllers.GitLab      reconciling Webservice and Sidekiq Deployments (paused) {"gitlab": "gitlab-system/gitlab"}
   ...
   ```

   위에 설명된 업그레이드 단계를 따르는 로그 항목이 표시됩니다. 클러스터에서 GitLab 사용자 정의 리소스 상태를 볼 수도 있습니다:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS        VERSION
   gitlab   Preparing     5.2.4
   ```

애플리케이션이 준비되고 새 버전으로 업그레이드되면 `STATUS` 열에 반영됩니다.

```shell
$ kubectl -n gitlab-system get gitlab
NAME     STATUS      VERSION
gitlab   Running     5.2.4
```

GitLab 객체 자체의 상태 조건은 애플리케이션에 대한 더 자세한 정보를 제공합니다.

### 다운타임과 함께 업그레이드 {#upgrade-with-downtime}

기본적으로 GitLab Operator는 [무중단 업그레이드](https://docs.gitlab.com/update/zero_downtime/) 경로를 적용하며, 이는 한 번에 하나의 부 버전을 업데이트하기 위해 필요합니다. 부 버전을 건너뛰고 싶다면(예: GitLab 18.0에서 18.2로 업그레이드), `gitlab.io/disable-zero-downtime-upgrade` 주석을 추가하여 무중단 업그레이드를 비활성화할 수 있습니다.

> [!warning]
> 다운타임을 포함한 업그레이드 중에 GitLab 인스턴스는 데이터베이스 마이그레이션이 실행되는 동안 사용할 수 없습니다. 진행하기 전에 유지 관리 창을 계획했으며 예상된 다운타임을 사용자에게 알렸는지 확인하세요.
>
> 부 버전을 건너뛸 때 필수 [업그레이드 중지](https://docs.gitlab.com/update/upgrade_paths/)를 계속 따라야 합니다. 각 필수 버전에서 중지한 후 대상 버전으로 진행되도록 업그레이드 경로를 계획하세요.

다운타임을 포함하여 업그레이드를 수행하려면:

1. [유지 관리 모드 켜기](https://docs.gitlab.com/administration/maintenance_mode/)를 고려하세요.
1. [백업](https://docs.gitlab.com/charts/backup-restore/)을 수행합니다.
1. GitLab Operator를 대상 GitLab 버전을 지원하는 버전으로 업그레이드합니다.
1. `gitlab.io/disable-zero-downtime-upgrade` 주석을 추가하고 GitLab 사용자 정의 리소스에서 `spec.chart.version`를 업데이트합니다:

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

1. 수정된 GitLab 사용자 정의 리소스를 적용합니다:

   ```shell
   kubectl -n gitlab-system apply -f mygitlab.yaml
   ```

   연산자는 자동으로:

   1. Webservice 및 Sidekiq 배포를 0개 복제본으로 축소합니다.
   1. 모든 pod가 종료될 때까지 대기합니다.
   1. 데이터베이스 마이그레이션을 실행합니다.
   1. Webservice 및 Sidekiq를 새 차트 버전과 조정합니다.
   1. 차트 값에서 Webservice 및 Sidekiq 복제본 개수를 복원합니다.

1. 업그레이드 진행 상황을 모니터링합니다:

   ```shell
   kubectl -n gitlab-system logs deployment/gitlab-controller-manager -c manager -f
   ```

1. 업그레이드가 완료될 때까지 기다립니다:

   ```shell
   $ kubectl -n gitlab-system get gitlab
   NAME     STATUS      VERSION
   gitlab   Running     9.3.0
   ```

1. 업그레이드가 완료된 후 주석을 제거하여 향후 업그레이드를 위한 기본 무중단 업그레이드 동작을 복원합니다:

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

1. 활성화된 경우 [유지 관리 모드 끄기](https://docs.gitlab.com/administration/maintenance_mode/#disable-maintenance-mode)

## GitLab Operator가 GitLab을 업그레이드하는 방법 {#how-gitlab-operator-upgrades-gitlab}

컨트롤러 조정 루프의 시작 부분에서 GitLab Operator는 현재 버전이 필수 버전과 일치하는지 확인합니다.

- 이 버전들이 일치하면 일반 조정 루프가 실행되어 사용자 정의 리소스(CR) 사양에 제공된 구성을 만족하는 객체가 존재하는지 확인합니다.
- 이 버전들이 일치하지 않으면 일반 조정 루프는 여전히 실행되지만 업그레이드를 처리하기 위해 논리의 추가 분기가 실행됩니다.

업그레이드에서:

1. 컨트롤러는 모든 배포를 조정합니다. Webservice 및 Sidekiq 배포는 조정되지만 일시 중지됩니다. 새 배포가 재개될 때까지 이전 pod는 계속 실행됩니다.
1. 사전 마이그레이션이 실행되며, 이는 마이그레이션 작업을 실행하지만 배포 후 마이그레이션을 건너뜁니다.
1. 컨트롤러는 Webservice 및 Sidekiq 배포를 재개합니다.
1. 컨트롤러는 새 Webservice 및 Sidekiq pod가 실행 중일 때까지 대기합니다.
1. 배포 후 마이그레이션은 배포 후 마이그레이션을 건너뛰지 않고 마이그레이션 작업을 실행합니다.
1. 컨트롤러는 Webservice 및 Sidekiq 배포에 대해 롤링 업데이트를 수행합니다.
1. 컨트롤러는 다시 시작된 Webservice 및 Sidekiq pod가 실행 중일 때까지 대기합니다.

향후 조정 루프에서는 원하는 버전(`spec.chart.version`에서)이 현재 버전(`status.version`에서)과 일치하기 때문에 논리의 이 분기를 건너뜁니다.

## 관련 주제 {#related-topics}

- [Helm 차트 설치 업그레이드](https://docs.gitlab.com/charts/installation/upgrade/)
- [GitLab Helm 차트 버전](https://docs.gitlab.com/charts/installation/version_mappings/)
- [업그레이드 경로 계획](https://docs.gitlab.com/update/upgrade_paths/)
- [GitLab 업그레이드 노트](https://docs.gitlab.com/update/versions/)
- [GitLab 버전 간의 변경 사항](https://gitlab-com.gitlab.io/cs-tools/gitlab-cs-tools/what-is-new-since/?tab=features)
