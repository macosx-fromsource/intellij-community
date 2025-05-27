---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 10. CI environments
---

Date: 2021-09-02

## Status

Accepted

## Context

[#238](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/238) was opened because if a job failed that happened prior to the jobs that deployed to our CI environments, the CI environment cleanup jobs would still trigger. This meant that those cleanup jobs would also fail, because there was no release to remove.

Also, if a developer manually retried one of the jobs that deploys to the cluster but forgot to eventually run the job that removes the release, then releases would be running indefinitely in the cluster and reduce capacity which would lead other pipelines to fail due to CPU/memory limitations.

To address this, we had a couple options:

1. Use `needs: [functional_tests_{x,y,z}]` on the cleanup jobs to ensure they only ran when those jobs were successful.
1. Use [GitLab CI environments](https://docs.gitlab.com/ci/environments/) to relate these jobs to each other around the concept of an "environment"

## Decision

Using [GitLab CI environments](https://docs.gitlab.com/ci/environments/) helps us conceptually tie together the jobs that deploy to an environment and the jobs that clean up an environment. This is the approach we use in Charts CI, and it ensures that releases are automatically cleaned up upon merge or after the specified `auto_stop_in` time period is met. This also ensures the cleanup jobs are only run in these situations since they are set to `when: manual`. Additionally, developers can "pin" the environment to prevent it from being automatically cleaned up in the event that they would like to investigate the release running in the cluster(s).

This approach gives us parity with other CI configurations used in the Distribution team, and also gives use additional functionality such as the aforementioned automatic cleanups and environment pinning.

## Consequences

CI environments will now be automatically cleaned up after the specified wait period, or upon the related MR being merged. This will help to ensure that releases are not left running unintentionally and indefinitely, helping with our CI cluster load overall.

Subsequently, use of explicit `needs` dependency from `cleanup` jobs on `test` or `qa` jobs tasks would be unnecessary.
