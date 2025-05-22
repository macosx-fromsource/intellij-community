---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Operator
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab Self-Managed

{{< /details >}}

The [GitLab Operator](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator) is an installation and management method that follows the
[Kubernetes Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

Use the GitLab Operator to run GitLab in
[OpenShift](https://docs.gitlab.com/install/openshift_and_gitlab/) or on
another Kubernetes-compatible platform.

{{< alert type="note" >}}

The GitLab Operator has [known limitations](#known-issues) and is only suitable for specific scenarios in production use.

{{< /alert >}}

<!-- This warning block is duplicated in doc/installation.md. Changes should be reflected in both locations. -->

{{< alert type="warning" >}}

The default values of the _GitLab custom resource_ are **not intended for production use**.
With these values, GitLab Operator creates a GitLab instance where _all_ services, including the persistent data,
are deployed in a Kubernetes cluster, which is **not suitable for production workloads**.
For production deployments, you **must** follow the [Cloud Native Hybrid reference architectures](https://docs.gitlab.com/administration/reference_architectures/#cloud-native-hybrid).
GitLab does not support any issues related to PostgreSQL, Redis, Gitaly, Praefect, or MinIO deployed inside of a Kubernetes Cluster.

{{< /alert >}}

## Known issues

GitLab Operator does not support:

- Migration from GitLab Chart or Linux package to GitLab Operator. For migration of the installation method, you must follow steps similar to the
  [manual migration steps](https://docs.gitlab.com/charts/installation/migration/package_to_helm/).
  Support for automatic migration is proposed in [GitLab Operator issue 1567](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1567).
- Git over SSH with [OpenShift routes](https://docs.openshift.com/container-platform/4.14/networking/routes/route-configuration.html).
  For more information, see [GitLab Operator documentation on OpenShift Routes](openshift_ingress.md#openshift-routes).
- [GKE workload identity](https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity) and [IAM service accounts](https://docs.aws.amazon.com/eks/latest/userguide/associate-service-account-role.html) to authenticate workloads to other cloud APIs (such as object storage).
  For more information, see [GitLab Operator issue 1089](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1737).

GitLab Operator has any other limitation of GitLab Chart. GitLab Operator relies on GitLab Chart to provision Kubernetes resources. Therefore, any limitation
in GitLab Chart impacts GitLab Operator. Removing the GitLab Chart dependency from GitLab Operator is proposed in
[Cloud Native epic 64](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/64).

## Installation

Instructions on how to install the GitLab Operator can be found in our [installation document](installation.md).

We list details of how we use
[Security Context Constraints](security_context_constraints.md) in their respective document.

You should also be aware of the [considerations for SSH access to Git](git_over_ssh.md), especially
when using OpenShift.

## Upgrading

[Operator upgrades](operator_upgrades.md) documentation demonstrates how to upgrade the GitLab Operator.

[GitLab upgrades](gitlab_upgrades.md) documentation demonstrates how to upgrade a GitLab instance, managed by the GitLab Operator.

## Backup and restore

[Backup and restore](backup_and_restore.md) documentation demonstrates how to back up and restore a GitLab instance that is managed by the Operator.

## Using RedHat certified images

[RedHat certified images](certified_images.md) documentation demonstrates how to instruct the GitLab Operator
to deploy images that have been certified by RedHat.

## Developer Tooling

- [Developer guide](developer/guide.md): Outlines the project structure and how to contribute.
- [Versioning and Release Info](developer/releases.md): Records notes concerning versioning and releasing the operator.
- [Design decisions](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr): This projects makes use of Architecture Decision Records, detailing the structure, functionality, and feature implementation of this Operator.
- [OpenShift Cluster Setup](developer/openshift_cluster_setup.md): Instructions for creating/configuring OpenShift clusters for *Development* purposes.
