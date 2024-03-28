# 20. OpenShift support policy

Date: 2024-03-26

Status: Accepted

## Context

The Distribution team needs to define how we support GitLab on various
OpenShift releases.

## Decision

GitLab will officially support four minor releases of OpenShift -- `N`, `N-1`,
and `N-2` and `N-3`. Like Kubernetes, `N` is either:

- The latest released minor version of OpenShift, if we have finished qualifying
  it.
- The next most recent version, if we have not finished or started qualifying
  the most recent version.

For example, if the current releases available are `4.14`, `4.13`, `4.12`,
`4.11` and we have not qualified release 4.15, then `N` would be `4.14` and we
would officially support releases `4.14`, `4.13`, `4.12`, and `4.11` as shown in
this table.

| Release | Reference |
|---------|-----------|
| `4.14`  | `N`       |
| `4.13`  | `N-1`     |
| `4.12`  | `N-2`     |
| `4.11`  | `N-2`     |

Details can be found at [Distribution Team Kubernetes and OpenShift release support policy](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)

## Consequences

- We can now direct interested third-parties to our support document when
  questions arise on what versions of OpenShift we support and how support them.
- Operator pipeline test jobs may have to be adjusted and/or added to conform
  with this policy.
