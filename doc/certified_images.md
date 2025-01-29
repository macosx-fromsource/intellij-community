---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: RedHat-certified images
---

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab Self-Managed

The following table lists the images that the GitLab Operator deploys. The table includes links to the
RedHat Technology Portal project listings where these images can be managed by GitLab team members.

The GitLab Operator image tags align with the
[GitLab Operator release versions](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/releases).

The NGINX Ingress Controller image tags align with the contents of the
[`TAG` file](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/-/blob/main/TAG) in the
[project fork](https://gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx) managed by GitLab.

The rest of the image tags follow `v<GitLab version>-ubi8` format, for example `v15.4.0-ubi8`. The tag suffix denotes
the images that have been built on top of the
[RedHat Universal Base Image (UBI)](https://catalog.redhat.com/software/containers/ubi8/ubi/5c359854d70cc534b3a3784e?container-tabs=overview),
a requirement for certification by RedHat. The GitLab Operator image itself only has one variant, which is already
built on top of UBI.

See the [Charts documentation on UBI images](https://docs.gitlab.com/charts/advanced/ubi/index.html)
for more information, including example Helm values to use these images.

Component | Registry path | RedHat Technology Portal
-|-|-
`gitlab-operator` | `registry.gitlab.com/gitlab-org/cloud-native/gitlab-operator:$OPERATOR_VERSION` | [Link](https://connect.redhat.com/component/629f9d952cb3e76438a9d40e/overview)
`gitlab-operator-bundle` | `registry.connect.redhat.com/gitlab/gitlab-operator-bundle` | [Link](https://connect.redhat.com/component/5f6cbaa04fcb1bc3f0425fbf/overview)
`alpine-certificates` | `registry.gitlab.com/gitlab-org/build/cng/alpine-certificates:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb615212977e7063dba93d0/overview)
`cfssl-self-sign` | `registry.gitlab.com/gitlab-org/build/cng/cfssl-self-sign:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/63474d9a673c3c4e34995d26/overview)
`kubectl` | `registry.gitlab.com/gitlab-org/build/cng/kubectl:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb611335e09a3c40183e67f/overview)
`gitaly` | `registry.gitlab.com/gitlab-org/build/cng/gitaly:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60ec6c65ee7c76a2ad0d8/overview)
`gitlab-container-registry` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-container-registry:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb611e7935e0609ade7b2cb/overview)
`gitlab-exporter` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-exporter:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)
`gitlab-geo-logcursor` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-geo-logcursor:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/630683e0290892d1ec194033/overview)
`gitlab-kas` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-kas:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/6306824c0d53878b3b4cc60d/overview)
`gitlab-mailroom` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-mailroom:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60e0a5e09a3c40183e67d/overview)
`gitlab-pages` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-pages:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/630683acb2ab2de150584661/overview)
`gitlab-shell` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-shell:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb57b5d3379deb31cba93e5/overview)
`gitlab-sidekiq-ee` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-sidekiq-ee:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60b4a2977e7063dba93cb/overview)
`gitlab-toolbox-ee` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-toolbox-ee:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/60fb728cc3450afa1bb969e2/overview)
`gitlab-webservice-ee` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-webservice-ee:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb607e4c65ee7c76a2ad0d4/overview)
`gitlab-workhorse-ee` | `registry.gitlab.com/gitlab-org/build/cng/gitlab-workhorse-ee:$GITLAB_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60c7b5e09a3c40183e67b/overview)
`gitlab-ingress-nginx` | `registry.gitlab.com/gitlab-org/cloud-native/charts/gitlab-ingress-nginx/controller:$NGINX_VERSION-ubi8` | [Link](https://connect.redhat.com/component/5fb60d575e09a3c40183e67c/overview)
