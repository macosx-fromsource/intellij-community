---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Operator

The GitLab Operator is an installation and management method that follows the
[Kubernetes Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

Use the GitLab Operator to run GitLab in
[OpenShift](https://docs.gitlab.com/ee/install/openshift_and_gitlab/index.html) or on
another Kubernetes-compatible platform.

## Installation

Instructions on how to install the GitLab Operator can be found our [installation document](installation.md).

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
