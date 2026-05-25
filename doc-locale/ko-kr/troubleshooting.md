---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Operator 문제 해결
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab Self-Managed

{{< /details >}}

이 문서는 GitLab Operator 설치 및 GitLab 사용자 정의 리소스에서 GitLab 인스턴스 배포를 문제 해결하는 데 도움이 되는 노트 및 팁의 모음입니다.

## 설치 문제 {#installation-problems}

Kubernetes 환경에서 operator 설치 문제 해결은 다른 Kubernetes 워크로드 문제 해결과 유사합니다. operator 매니페스트를 배포한 후 operator Pod의 `kubectl describe` 또는 `kubectl get events -n <namespace>`의 출력을 모니터링하세요. 이는 operator 이미지 검색 또는 operator 시작을 위한 다른 전제 조건의 문제를 나타냅니다.

operator가 시작되지만 조기에 종료되는 경우, operator 로그를 검사하면 Pod 종료 원인을 판단하기 위한 정보를 제공할 수 있습니다. 이는 다음 명령으로 수행할 수 있습니다:

```shell
kubectl logs deployment/gitlab-controller-manager -c manager -f -n <namespace>
```

또한 operator는 적절한 작동을 위해 TLS 인증서를 생성하기 위해 Cert Manager에 의존합니다. TLS 인증서는 Secret으로 생성되고 operator Pod에서 볼륨으로 탑재됩니다. TLS 인증서 획득 문제는 네임스페이스의 이벤트 로그에서 찾을 수 있습니다.

```shell
$ kubectl get events -n gitlab-system
...
102s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    MountVolume.SetUp failed for volume "cert" : secret "webhook-server-cert" not found
107s        Warning   FailedMount         pod/gitlab-controller-manager-d4f65f856-b4mdj    Unable to attach or mount volumes: unmounted volumes=[cert], unattached volumes=[cert gitlab-manager-token-fc4p9]: timed out waiting for the condition
...
```

다음 단계는 TLS 인증서 생성 실패를 나타내는 문제를 찾기 위해 Cert Manager 로그를 검사하는 것입니다.

### OpenShift 특정 문제 {#openshift-specific-problems}

OpenShift는 보다 제한적인 보안 모델을 가지고 있으며, 그 결과 GitLab operator는 클러스터 관리자 계정으로 설치해야 합니다. 개발자 계정에는 operator가 제대로 작동할 수 있는 필요한 권한이 없습니다.

[Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) Pod이 [이 문제](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/762)에 설명된 대로 잘못된 SCC 매개변수로 인해 프로비저닝할 수 없는 경우, 적절한 해결 방법은 리포지토리에서 SCC를 업데이트하여 OpenShift 클러스터에서 Ingress NGINX를 시작할 수 있도록 하는 것입니다:

1. GitLab Operator의 [최신 OpenShift 매니페스트](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)를 가져오세요. `gitlab-operator-openshift-VERSION.yaml`가 필요합니다
1. SCC 추출

   ```shell
   yq eval '. | select(.metadata.name | test(".*scc.*"))' gitlab-operator-openshift-VERSION.yaml > scc.yaml
   ```

1. `scc.yaml`을 클러스터에 적용하세요:

   ```shell
   kubectl apply -f scc.yaml
   ```

GitLab Operator 리포지토리의 Releases 페이지에서 릴리스된 매니페스트에서 설치하면 SCC가 포함되어 있으므로 이 문제가 없을 것입니다. OperatorHubs에서 지원되지 않는 객체와 관련된 문제:

- [IngressClass CR에 대한 지원 추가 · Issue #5491 · operator-framework/operator-sdk · GitHub](https://github.com/operator-framework/operator-sdk/issues/5491)
- [OpenShift의 SCC에 대한 지원 추가 · Issue #2847 · operator-framework/operator-lifecycle-manager · GitHub](https://github.com/operator-framework/operator-lifecycle-manager/issues/2847)

## GitLab 인스턴스 배포 문제 {#problems-with-deployment-of-gitlab-instance}

여기에 제시된 정보 외에도, GitLab Helm 차트 [문제 해결 문서](https://docs.gitlab.com/charts/troubleshooting/)를 참조해야 합니다.

### 핵심 서비스 준비 안 됨 {#core-services-not-ready}

GitLab Operator는 Redis, PostgreSQL 및 Gitaly의 인스턴스 설치에 의존하며, 이를 핵심 서비스라고 합니다. GitLab 고객 리소스를 배포한 후 핵심 서비스가 준비되지 않았다는 operator 로그 메시지가 과도하게 많으면, 이러한 서비스 중 하나가 작동 문제를 겪고 있는 것입니다.

구체적으로 각 서비스의 엔드포인트를 확인하여 서비스의 Pod에 연결되고 있는지 확인하세요. 이는 또한 클러스터에 GitLab 인스턴스를 지원할 충분한 리소스가 없으며 클러스터에 추가 노드를 추가해야 한다는 가능한 표시입니다.

[\#305](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/305) 문제가 GitLab 인스턴스의 배포를 중지하는 핵심 서비스를 보고하는 것을 추적하기 위해 생성되었습니다.

### GitLab UI에 도달할 수 없음(Ingresses가 주소를 가지지 않음 및/또는 CertManager 문제 실패) {#gitlab-ui-unreachable-ingresses-have-no-address-andor-certmanager-challenges-failing}

GitLab Operator의 설치 매니페스트 및 Helm 차트는 Helm 값에 `nameOverride`가 지정되지 않으면 기본적으로 모든 리소스 이름의 접두사로 `gitlab`을 사용합니다.

그 결과, NGINX IngressClass의 이름은 `gitlab-nginx`입니다. `gitlab` 이외의 릴리스 이름이 `metadata.name` 아래의 GitLab 사용자 정의 리소스에 지정된 경우, 기본 IngressClass 이름을 `global.ingress.class` 아래에 명시적으로 설정해야 합니다:

예를 들어, `metadata.name`이 `demo`로 설정되면, `global.ingress.class=gitlab-nginx`을 설정하세요:

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

이 명시적 설정 없이는 Ingresses가 `demo-nginx`라는 이름의 Ingress를 찾으려고 시도하며, 이는 존재하지 않습니다.

### NGINX Ingress Controller Pod 누락 {#nginx-ingress-controller-pods-missing}

OpenShift 환경에서 [NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)는 GitLab 인스턴스(HTTPS 및 SSH 모두)로 트래픽을 지향하기 위해 OpenShift Routes 대신 사용됩니다. GitLab 인스턴스 연결에 문제가 있는 경우 먼저 NGINX Ingress Controller에 대한 배포가 있는지 확인하세요.

배포가 있으면 `kubectl get deploy` 출력의 `READY` 열을 확인하세요. `READY` 상태가 `0/0`로 보고되면, Security Context Constraint (SCC)가 위반되었음을 나타내는 메시지를 찾기 위해 `kubectl get events -n <namespace> | grep -i nginx`의 출력을 검사하세요.

이는 OpenShift용 NGINX RBAC 리소스가 배포되지 않았음을 나타냅니다. OpenShift용 operator 매니페스트는 다음 명령으로 재적용되어야 합니다:

```shell
kubectl apply -f https://gitlab.com/api/v4/projects/18899486/packages/generic/gitlab-operator/<VERSION>/gitlab-operator-openshift.yaml
```

매니페스트가 적용된 후에는 SCC를 적절히 획득하고 Ingress 컨트롤러가 Pod을 올바르게 생성할 수 있도록 하기 위해 Ingress 컨트롤러 배포를 삭제해야 할 수 있습니다.

### 수평 Pod 자동 스케일러가 스케일링되지 않음 {#horizontal-pod-autoscalers-are-not-scaling}

수평 Pod 자동 스케일러(HPA)가 트래픽 로드에 따라 Pod 수를 스케일링하지 않는 것으로 발견되면, Metrics Server의 설치를 확인하세요. Kubernetes 클러스터에서 Metrics Server는 설치해야 하는 추가 구성 요소입니다. 설치 프로세스는 [설치 문서](installation.md#metrics)에서 찾을 수 있습니다.

OpenShift 클러스터에는 기본 제공 Metrics Server가 있으므로 그 결과 HPAs는 올바르게 작동해야 합니다.

### PersistentVolumeClaim 구성이 변경될 때 데이터 복원 {#restoring-data-when-persistentvolumeclaim-configuration-changes}

데이터 지속성을 위해 MinIO와 같은 구성 요소로 작업할 때, 이전 PersistentVolume에 다시 연결해야 할 수도 있습니다.

예를 들어, [!419](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/merge_requests/419)는 Operator 정의 MinIO 구성 요소를 GitLab Helm 차트의 MinIO 구성 요소로 바꾸었습니다. 이 변경의 일부로 PersistentVolumeClaim을 포함한 객체 이름이 변경되었습니다. 결과적으로, Operator 번들 MinIO 인스턴스를 사용하는 모든 사람이 지속된 데이터를 포함하는 이전 PersistentVolume에 다시 연결하기 위해 추가 단계를 취해야 했습니다.

GitLab Operator `0.6.4`로 업그레이드한 후, 새 PersistentVolumeClaim을 이전 PersistentVolume에 연결하려면 다음 단계를 완료하세요:

1. `$RELEASE_NAME-minio-secret` Secret을 삭제하세요. Secret의 내용은 `0.6.4` 업그레이드로 변경되지만, Secret 이름은 변경되지 않습니다.
1. 이전 MinIO PersistentVolume을 편집하여 `.spec.persistentVolumeReclaimPolicy`을 `Delete`에서 `Retain`로 변경하세요.
1. 이전 MinIO StatefulSet, `$RELEASE_NAME-minio`을 삭제하세요.
1. 이전 MinIO PersistentVolume에서 `.spec.ClaimRef`을 제거하여 이전 MinIO PersistentVolumeClaim과의 연결을 끊으세요.
1. 이전 MinIO PersistentVolumeClaim, `export-gitlab-minio-0`을 삭제하세요.
1. 이전 PersistentVolume 상태가 이제 `Available`인지 확인하세요.
1. GitLab 사용자 정의 리소스에서 다음 값을 설정하세요: `minio.persistence.volumeName=<previous PersistentVolume name>`.
1. GitLab 사용자 정의 리소스를 적용하세요.
1. 새 MinIO PersistentVolumeClaim(및 MinIO Pod, PersistentVolumeClaim이 바인딩 해제되고 삭제될 수 있도록)을 삭제하세요. Operator가 PersistentVolumeClaim을 다시 생성합니다. 이는 `.spec` 필드가 변경 불가능하기 때문에 필요합니다.
1. 이전 MinIO PersistentVolume이 이제 새 MinIO PersistentVolumeClaim에 바인딩되어 있는지 확인하세요.
1. GitLab UI에서 이슈, 아티팩트 등으로 이동하여 데이터가 복원되었는지 확인하세요.

이전 PersistentVolumes에 다시 연결하는 것에 대한 자세한 내용은 [지속 볼륨 문서](https://docs.gitlab.com/charts/advanced/persistent-volumes/)를 참조하세요.

상기로, 번들 MinIO 인스턴스는 [프로덕션 사용을 위해 권장되지 않습니다](https://docs.gitlab.com/charts/charts/minio/#enable-the-sub-chart).

### 여러 데이터베이스 연결 구성 {#configure-multiple-database-connections}

GitLab 16.0에서 GitLab은 기본적으로 동일한 PostgreSQL 데이터베이스를 가리키는 두 개의 데이터베이스 연결을 사용합니다.

단일 데이터베이스 연결로 다시 전환하려면, [여러 데이터베이스 연결 구성](https://docs.gitlab.com/charts/charts/globals/#configure-multiple-database-connections)을 참조하세요.

### 구성 요소 비활성화 또는 이름 바꾸기 {#disabling-or-renaming-components}

`nameOverride` 변경 및 다양한 `*.enable: false` 값 조합을 통한 리소스의 이름 바꾸기 및 비활성화는 가능하지만, GitLab Operator는 더 이상 필요하지 않은 Kubernetes 리소스를 자동으로 제거하지 않습니다. 그 결과, 위의 작업 중 하나라도 남은 리소스의 수동 관리가 필요합니다.

그러나 GitLab 사용자 정의 리소스의 인스턴스를 삭제하면 예상대로 해당 인스턴스와 관련된 모든 리소스가 제거됩니다.

[!889](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/889) 문제가 이를 추적하기 위해 생성되었습니다.
