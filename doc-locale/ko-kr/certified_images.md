---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: RedHat 인증 이미지
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

다음 테이블에는 GitLab Operator가 배포하는 이미지 목록이 있습니다. GitLab 팀 멤버가 관리할 수 있는 이미지의 RedHat Technology Portal 프로젝트 목록 링크도 포함되어 있습니다.

GitLab Operator 이미지 태그는
[GitLab Operator 릴리즈 버전](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases)과 일치합니다.

NGINX Ingress Controller 이미지 태그는 GitLab이 관리하는
[프로젝트 포크](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx)의
[`TAG` 파일](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/-/blob/main/TAG) 내용과 일치합니다.

나머지 이미지 태그는 `v<GitLab version>-ubi8` 형식을 따릅니다(예: `v15.4.0-ubi8`). 태그 접미사는 RedHat 인증 요구 사항인
[RedHat Universal Base Image (UBI)](https://catalog.redhat.com/software/containers/ubi8/ubi/5c359854d70cc534b3a3784e?container-tabs=overview)
위에 빌드된 이미지를 나타냅니다. GitLab Operator 이미지 자체는 이미 UBI 위에 빌드된 단일 변형만 있습니다.

이러한 이미지에 사용할 Helm 값 예시를 포함한 자세한 내용은
[UBI 이미지에 관한 Charts 문서](https://docs.gitlab.com/charts/advanced/ubi/)를 참조하세요.

| 컴포넌트                                                                                             | 레지스트리 경로 |
|-------------------------------------------------------------------------------------------------------|---------------|
| [`gitlab-operator`](https://connect.redhat.com/component/629f9d952cb3e76438a9d40e/overview)           | `registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:$OPERATOR_VERSION` |
| [`gitlab-operator-bundle`](https://connect.redhat.com/component/5f6cbaa04fcb1bc3f0425fbf/overview)    | `registry.connect.redhat.com/gitlab/gitlab-operator-bundle` |
| [`alpine-certificates`](https://connect.redhat.com/component/5fb615212977e7063dba93d0/overview)       | `registry.gitlab.com/gitlab-org/build/cng/alpine-certificates:$GITLAB_VERSION-ubi` |
| [`cfssl-self-sign`](https://connect.redhat.com/component/63474d9a673c3c4e34995d26/overview)           | `registry.gitlab.com/gitlab-org/build/cng/cfssl-self-sign:$GITLAB_VERSION-ubi` |
| [`kubectl`](https://connect.redhat.com/component/5fb611335e09a3c40183e67f/overview)                   | `registry.gitlab.com/gitlab-org/build/cng/kubectl:$GITLAB_VERSION-ubi` |
| [`gitaly`](https://connect.redhat.com/component/5fb60ec6c65ee7c76a2ad0d8/overview)                    | `registry.gitlab.com/gitlab-org/build/cng/gitaly:$GITLAB_VERSION-ubi` |
| [`gitlab-container-registry`](https://connect.redhat.com/component/5fb611e7935e0609ade7b2cb/overview) | `registry.gitlab.com/gitlab-org/build/cng/gitlab-container-registry:$GITLAB_VERSION-ubi` |
| [`gitlab-exporter`](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)           | `registry.gitlab.com/gitlab-org/build/cng/gitlab-exporter:$GITLAB_VERSION-ubi` |
| [`gitlab-geo-logcursor`](https://connect.redhat.com/component/630683e0290892d1ec194033/overview)      | `registry.gitlab.com/gitlab-org/build/cng/gitlab-geo-logcursor:$GITLAB_VERSION-ubi` |
| [`gitlab-kas`](https://connect.redhat.com/component/6306824c0d53878b3b4cc60d/overview)                | `registry.gitlab.com/gitlab-org/build/cng/gitlab-kas:$GITLAB_VERSION-ubi` |
| [`gitlab-mailroom`](https://connect.redhat.com/component/5fb60e0a5e09a3c40183e67d/overview)           | `registry.gitlab.com/gitlab-org/build/cng/gitlab-mailroom:$GITLAB_VERSION-ubi` |
| [`gitlab-pages`](https://connect.redhat.com/component/630683acb2ab2de150584661/overview)              | `registry.gitlab.com/gitlab-org/build/cng/gitlab-pages:$GITLAB_VERSION-ubi` |
| [`gitlab-shell`](https://connect.redhat.com/component/5fb57b5d3379deb31cba93e5/overview)              | `registry.gitlab.com/gitlab-org/build/cng/gitlab-shell:$GITLAB_VERSION-ubi` |
| [`gitlab-sidekiq-ee`](https://connect.redhat.com/component/5fb60b4a2977e7063dba93cb/overview)         | `registry.gitlab.com/gitlab-org/build/cng/gitlab-sidekiq-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-toolbox-ee`](https://connect.redhat.com/component/60fb728cc3450afa1bb969e2/overview)         | `registry.gitlab.com/gitlab-org/build/cng/gitlab-toolbox-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-webservice-ee`](https://connect.redhat.com/component/5fb607e4c65ee7c76a2ad0d4/overview)      | `registry.gitlab.com/gitlab-org/build/cng/gitlab-webservice-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-workhorse-ee`](https://connect.redhat.com/component/5fb60c7b5e09a3c40183e67b/overview)       | `registry.gitlab.com/gitlab-org/build/cng/gitlab-workhorse-ee:$GITLAB_VERSION-ubi` |
| [`gitlab-ingress-nginx`](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)      | `registry.gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/controller:$NGINX_VERSION-ubi` |

## 이미지 서명 {#image-signatures}

Operator 이미지는 cosign으로 검증할 수 있습니다.

```script
cosign verify "registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:$OPERATOR_VERSION" \
  --certificate-identity "https://gitlab.com/gitlab-org/cloud-native/gitlab-operator//.gitlab-ci.yml@refs/heads/$OPERATOR_VERSION" \
  --certificate-oidc-issuer "https://gitlab.com"
```

CNG 이미지 서명 검증 방법은 [Helm 차트 문서](https://docs.gitlab.com/charts/installation/verify_cng_images/)에서 확인할 수 있습니다.
