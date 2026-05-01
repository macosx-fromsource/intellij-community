---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Operator 문제 해결
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

이 문서는 GitLab Operator 설치 및 GitLab 커스텀 리소스에서 GitLab 인스턴스를 배포할 때 발생하는 문제를 해결하는 데 도움이 되는 참고 사항과 팁을 모아 놓은 것입니다.

## 설치 문제 {#installation-problems}

Kubernetes 환경에서 Operator 설치 문제를 해결하는 방법은 다른 Kubernetes 워크로드의 문제 해결 방법과 크게 다르지 않습니다. Operator 매니페스트를 배포한 후 Operator Pod에 대해 `kubectl describe`를 실행하거나 `kubectl get events -n <namespace>`를 실행하여 출력을 모니터링하세요. 이를 통해 Operator 이미지를 가져오는 데 문제가 있거나 Operator 시작을 위한 사전 조건에 문제가 있는지 확인할 수 있습니다.

Operator가 시작되었지만 비정상적으로 종료되는 경우, Operator 로그를 확인하면 Pod 종료 원인을 파악하는 데 도움이 됩니다. 다음 명령어를 사용하세요:

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

또한 Operator는 정상 작동을 위해 TLS 인증서를 생성하는 Cert Manager에 의존합니다. TLS 인증서는 Secret으로 생성되어 Operator Pod에 볼륨으로 마운트됩니다. TLS 인증서 획득 관련 문제는 네임스페이스의 이벤트 로그에서 확인할 수 있습니다.

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

다음 단계로 Cert Manager 로그를 검사하여 TLS 인증서 생성 실패와 관련된 문제를 확인하세요.

### OpenShift 관련 문제 {#openshift-specific-problems}

OpenShift는 보다 엄격한 보안 모델을 적용하므로 GitLab Operator는 클러스터 관리자 계정으로 설치해야 합니다. 개발자 계정에는 Operator가 올바르게 작동하는 데 필요한 권한이 없습니다.

[Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) Pod가 [이 이슈](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762)에 설명된 것처럼 잘못된 SCC 파라미터로 인해 프로비저닝되지 않는 경우, 올바른 해결 방법은 리포지토리에서 SCC를 업데이트하여 OpenShift 클러스터에서 Ingress NGINX가 시작될 수 있도록 하는 것입니다:

1. GitLab Operator의 [최신 OpenShift 매니페스트](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)를 가져옵니다. `gitlab-operator-openshift-VERSION.yaml` 파일이 필요합니다.
1. SCC를 추출합니다.

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. 클러스터에 `scc.yaml`을 적용합니다:

   ```shell
   kubectl apply -f scc.yaml
   ```

GitLab Operator 리포지토리의 Releases 페이지에서 릴리즈된 매니페스트를 설치하면 SCC가 포함되어 있으므로 이 문제가 발생하지 않습니다.
OperatorHub에서 지원되지 않는 오브젝트 관련 이슈:

- [Add support for IngressClass CR · Issue #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [Add support for OpenShift's SCC · Issue #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## GitLab 인스턴스 배포 문제 {#problems-with-deployment-of-gitlab-instance}

여기에 제공된 정보 외에도 GitLab Helm 차트 [문제 해결 문서](https://docs.gitlab.com/charts/troubleshooting/)를 참조하세요.

### 핵심 서비스 준비되지 않음 {#core-services-not-ready}

GitLab Operator는 핵심 서비스로 알려진 Redis, PostgreSQL, Gitaly 인스턴스 설치에 의존합니다. GitLab 커스텀 리소스를 배포한 후 핵심 서비스가 준비되지 않았다는 Operator 로그 메시지가 과도하게 발생하는 경우, 해당 서비스 중 하나가 정상적으로 작동하는 데 문제가 있는 것입니다.

각 서비스의 엔드포인트를 확인하여 서비스의 Pod에 연결되고 있는지 확인하세요. 이는 클러스터에 GitLab 인스턴스를 지원할 충분한 자원이 없어 클러스터에 노드를 추가해야 한다는 신호일 수도 있습니다.

이슈 [#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305)는 GitLab 인스턴스 배포를 중단시키는 핵심 서비스 보고를 추적하기 위해 생성되었습니다.

### GitLab UI에 접근할 수 없음 (Ingress에 주소가 없거나 CertManager 챌린지 실패) {#gitlab-ui-unreachable-ingresses-have-no-address-and-or-certmanager-challenges-failing}

GitLab Operator의 설치 매니페스트와 Helm 차트는 Helm 값에 `nameOverride`가 지정되지 않는 한 기본적으로 모든 리소스 이름의 접두사로 `gitlab`을 사용합니다.

따라서 NGINX IngressClass의 이름은 `gitlab-nginx`가 됩니다. GitLab 커스텀 리소스의 `metadata.name`에 `gitlab` 이외의 릴리즈 이름이 지정된 경우, `global.ingress.class`에 기본 IngressClass 이름을 명시적으로 설정해야 합니다:

예를 들어 `metadata.name`이 `demo`로 설정된 경우, `global.ingress.class=gitlab-nginx`로 설정합니다:

```yaml
apiVersion: apps.gitlab.com/v1beta1
kind: GitLab
metadata:
  name: demo
spec:
  chart:
    version: "X.Y.Z"
    values:
      global:
        ingress:
          # Use the correct IngressClass name.
          class: gitlab-nginx
```

이 설정을 명시적으로 지정하지 않으면 Ingress가 존재하지 않는 `demo-nginx`라는 이름의 Ingress를 찾으려 합니다.

### NGINX Ingress Controller Pod 누락 {#nginx-ingress-controller-pods-missing}

OpenShift 환경에서는 GitLab 인스턴스로의 트래픽 라우팅(HTTPS 및 SSH 모두)을 위해 OpenShift Routes 대신 [NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)가 사용됩니다. GitLab 인스턴스 연결에 문제가 있는 경우, 먼저 NGINX Ingress Controller 배포가 존재하는지 확인하세요.

배포가 존재하는 경우, `kubectl get deploy` 출력의 `READY` 열을 확인하세요. `READY` 상태가 `0/0`으로 표시되면 `kubectl get events -n <namespace> | grep -i nginx` 출력을 검사하여 SCC(Security Context Constraint)가 위반되었다는 메시지를 찾아보세요.

이는 OpenShift용 NGINX RBAC 리소스가 배포되지 않았음을 나타냅니다. 다음 명령어를 사용하여 OpenShift용 Operator 매니페스트를 다시 적용해야 합니다:

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

매니페스트가 적용된 후 SCC를 올바르게 획득하고 Ingress Controller가 Pod를 정상적으로 생성할 수 있도록 Ingress Controller 배포를 삭제해야 할 수 있습니다.

### 수평 Pod 자동 스케일러가 스케일링되지 않음 {#horizontal-pod-autoscalers-are-not-scaling}

HPA(수평 Pod 자동 스케일러)가 트래픽 부하에 따라 Pod 수를 스케일링하지 않는 경우, Metrics Server 설치 여부를 확인하세요. Kubernetes 클러스터에서 Metrics Server는 별도로 설치해야 하는 추가 구성 요소입니다. 설치 방법은 [설치 문서](installation.md#metrics)에서 확인할 수 있습니다.

OpenShift 클러스터에는 Metrics Server가 내장되어 있으므로 HPA가 올바르게 작동해야 합니다.

### PersistentVolumeClaim 구성 변경 시 데이터 복원 {#restoring-data-when-persistentvolumeclaim-configuration-changes}

MinIO와 같은 구성 요소를 데이터 영속성에 사용할 때 이전 PersistentVolume에 다시 연결해야 하는 경우가 있습니다.

예를 들어 [!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)에서는 Operator에서 정의한 MinIO 구성 요소를 GitLab Helm 차트의 MinIO 구성 요소로 교체했습니다. 이 변경으로 인해 PersistentVolumeClaim을 포함한 오브젝트 이름이 변경되었습니다. 따라서 Operator에 번들된 MinIO 인스턴스를 사용하는 경우 영속 데이터가 포함된 이전 PersistentVolume에 다시 연결하기 위한 추가 단계가 필요했습니다.

GitLab Operator `0.6.4`로 업그레이드한 후 다음 단계를 완료하여 새 PersistentVolumeClaim을 이전 PersistentVolume에 연결하세요:

1. `$RELEASE_NAME-minio-secret` Secret을 삭제합니다. Secret의 내용은 `0.6.4` 업그레이드와 함께 변경되지만 Secret 이름은 변경되지 않습니다.
1. 이전 MinIO PersistentVolume을 편집하여 `.spec.persistentVolumeReclaimPolicy`를 `Delete`에서 `Retain`으로 변경합니다.
1. 이전 MinIO StatefulSet인 `$RELEASE_NAME-minio`를 삭제합니다.
1. 이전 MinIO PersistentVolume에서 `.spec.ClaimRef`를 제거하여 이전 MinIO PersistentVolumeClaim과의 연결을 해제합니다.
1. 이전 MinIO PersistentVolumeClaim인 `export-gitlab-minio-0`을 삭제합니다.
1. 이전 PersistentVolume 상태가 이제 `Available`인지 확인합니다.
1. GitLab 커스텀 리소스에 다음 값을 설정합니다: `minio.persistence.volumeName=<이전 PersistentVolume 이름>`.
1. GitLab 커스텀 리소스를 적용합니다.
1. 새 MinIO PersistentVolumeClaim(및 MinIO Pod)을 삭제하여 PersistentVolumeClaim의 바인딩을 해제하고 삭제할 수 있도록 합니다. Operator가 PersistentVolumeClaim을 다시 생성합니다. `.spec` 필드는 변경 불가능하므로 이 단계가 필요합니다.
1. 이전 MinIO PersistentVolume이 새 MinIO PersistentVolumeClaim에 바인딩되었는지 확인합니다.
1. GitLab UI에서 이슈, 아티팩트 등으로 이동하여 데이터가 복원되었는지 확인합니다.

이전 PersistentVolume에 다시 연결하는 방법에 대한 자세한 내용은 [영속 볼륨 문서](https://docs.gitlab.com/charts/advanced/persistent-volumes/)를 참조하세요.

번들된 MinIO 인스턴스는 [프로덕션 사용에 권장되지 않습니다](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart).

### 다중 데이터베이스 연결 구성 {#configure-multiple-database-connections}

GitLab 16.0에서 GitLab은 기본적으로 동일한 PostgreSQL 데이터베이스를 가리키는 두 개의 데이터베이스 연결을 사용합니다.

단일 데이터베이스 연결로 되돌리려면 [다중 데이터베이스 연결 구성](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections)을 참조하세요.

### 구성 요소 비활성화 또는 이름 변경 {#disabling-or-renaming-components}

`nameOverride` 변경 및 다양한 `*.enable: false` 값의 조합을 통해 리소스의 이름 변경 및 비활성화가 가능하지만, GitLab Operator는 더 이상 필요하지 않은 Kubernetes 리소스를 자동으로 제거하지 않습니다. 따라서 위의 작업을 수행하면 남은 리소스를 수동으로 관리해야 합니다.

그러나 GitLab 커스텀 리소스의 인스턴스를 삭제하면 해당 인스턴스와 연결된 모든 리소스가 예상대로 제거됩니다.

이슈 [!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889)는 이를 추적하기 위해 생성되었습니다.
