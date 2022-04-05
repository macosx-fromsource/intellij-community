# 14. CI pipelines in dev

Date: 2021-12-07

## Status

Accepted

## Context

Issue [#448](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/448) discussed the possibility
of skipping "review" and "qa" stages for pipelines in the `dev` GitLab instance. This was proposed because
running these stages adds complexity and resource costs, so we wanted to be sure we were getting enough value
out of that configuration.

## Decision

1. Pipelines on `dev` will only run for `stable` branches, release tags, and (manually-pushed) feature branches.
1. Pipelines on `dev` for `stable` branches will run the tests against the CI clusters.
1. Pipelines on `dev` for release tags will _not_ run the tests against the CI clusters.
1. Pipelines on `dev` for feature branches will happen when the branch is manually pushed to `dev`, and will run the tests against CI clusters.

## Consequences

1. Pipelines on `dev` will no longer run for commits to `master`.
1. Cluster resources on `dev` will be under less load due to fewer review environments.
1. Pipelines on `dev` will need to be fixed by adding an image pull secret because the `dev` GitLab instance
   (and therefore its container registry) is private ([#449](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/issues/449)).
