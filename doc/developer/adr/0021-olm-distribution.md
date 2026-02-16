---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 21. Distribution via OLM
---

Date: 2024-04-18

## Context

The Distribution team needs to decide whether or not to distribute
the GitLab Operator via Operator Lifecycle Manager (OLM).

Discussion available in [issue 1562](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/1562).

## Decision

GitLab will continue to distribute the GitLab Operator via OLM for the
[inititial production readiness stage](https://gitlab.com/groups/gitlab-org/cloud-native/-/epics/68).
However, it will be marked as experimental and unsupported until:

- There is a clear method to upgrade GitLab following the required upgrade path when
  deployed via OLM.
- The team is able to automate releases of the GitLab Operator to OLM without manual human intervention.
- OLM-specific complications outlined in
  [issue 241](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/241)
  are addressed.

## Consequences

- The recommended methods for installing the GitLab Operator in Kubernetes and OpenShift
  are the Kubernetes manifest file or the GitLab Operator Helm chart.
- The documentation and the Operator's product page in OLM
  will communicate the recommended installation methods to use instead of
  using OLM.
- GitLab does not support any issues related to instances deployed using OLM.
- When OLM is considered fully supported, the documentation and OLM product
  page will be updated to include OLM as a recommended installation method.
