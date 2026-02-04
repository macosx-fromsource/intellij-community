---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 11. Operator release process
---

Date: 2021-09-09

## Status

Accepted

## Context

We need to integrate releases of operator to our official release tooling, which
is mostly driven by [release-tools](https://gitlab.com/gitlab-org/release-tools),
so that operator versions can be released as part of official GitLab releases.

## Decision

The following iterations are proposed to implement an automatic release of
operator

1. A mirror of operator project will be setup in `dev.gitlab.org` - [#286](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/286)
1. CI jobs that are relevant for development will be run only on the .com
   project, while jobs necessary to confirm that we are shipping a working
   artifact will be run also on the dev mirror.
1. Release will happen through Git tag pushes, and pipelines caused by them.
1. Tag pipeline in both .com project and its dev mirror will have two phases -
   tag and publish.
1. In .com tag pipeline
   1. Tag phase will perform the steps other than building an artifact and
      deploying it to a registry. This involves jobs like static analysis,
      tests that does not require artifacts, etc.
   1. Publish phase will build the artifacts and deploy it to a public
      registry.
   1. First job of the publish phase will be a blocking manual job, which when
      played will cause the pipeline to continue automatically and complete the
      release.
1. In dev tag pipeline
   1. Tag phase will build and deploy artifacts to an internal registry.
   1. These can be used to verify whether the artifacts we are about to release
      work as expected. We may eventually add a test infrastructure to dev to
      automate this testing.
   1. Release phase will have a manual job, which when played will trigger the
      manual job (part of release phase) in the corresponding tag pipeline in
      .com project.
1. For the first iteration and GA release, maintainers will manually perform the
   following tasks
   1. Update [`CHART_VERSIONS`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/CHART_VERSIONS) file.
   1. Create a Git tag and push it to both .com project and the dev mirror.
   1. Verify the operator artifacts created by the dev mirror works as expected.
   1. Once GitLab is released, play the manual job in the dev tag pipeline to
      complete the release.
   1. Ensure the tag pipeline in .com project completed and the artifacts are
      available to the users from public registries.
1. As the next iteration, we will implement the stable branch strategy in
   operator. This will essentially mean a stable branch will be cut from the
   default branch towards the end of the milestone (or when a new operator
   release is required), and tags will be created from that stable branch.
1. As the next iteration, we will implement an automated way to update the
   `CHART_VERSIONS` file.
1. As the next iteration, we will hand over the tasks to `release-tools` -
   [#224](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/224)
1. Because any [version of operator is expected to support latest three minor versions of GitLab chart](0006-gitlab-application-versions-supported.md),
   to avoid redundant operator versions, `ONLY THE RELEASE OF LATEST STABLE
   VERSION OF GITLAB CHARTS WILL CAUSE A NEW OPERATOR VERSION TO BE RELEASED
   AUTOMATICALLY`.
   - For example, if a patch release for GitLab chart 5.4.x, 5.3.x, and 5.2.x
     is being done, only 5.4.x will cause a new operator release.

## Consequences

1. For the time being, release will still be manual by maintainers updating
   `CHART_VERSIONS` file and creating a Git tag, and then finally playing the
   manual job to start the release.
