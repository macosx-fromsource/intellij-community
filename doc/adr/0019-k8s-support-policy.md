# 19. Kubernetes support policy

Date: 2024-03-26

Status: Accepted

## Context

The Distribution team needs to define how we support GitLab on various
Kubernetes releases.

## Decision

GitLab will officially support three minor releases of Kubernetes: `N`, `N-1`,
and `N-2`. `N` is either:

- The latest released minor version of Kubernetes, if we have finished
  qualifying it.
- The next most recent version, if we have not finished or started qualifying
  the most recent version.

For example, if the current releases available are `1.28`, `1.27`, `1.26`,
`1.25` and we have not qualified release `1.28`, then `N` would be `1.27` and we
would officially support releases `1.25`, `1.26`, and `1.27` as shown in this
table.

| Release | Reference |
|---------|-----------|
| `1.27`  | `N`       |
| `1.26`  | `N-1`     |
| `1.25`  | `N-2`     |

Details can be found at [Distribution Team Kubernetes and OpenShift release support policy](https://handbook.gitlab.com/handbook/engineering/infrastructure/core-platform/systems/distribution/k8s-release-support-policy/)

## Consequences

- We can now direct interested third-parties to our support document when
  questions arise on what versions of Kubernetes we support and how support them.
- Operator pipeline test jobs may have to be adjusted and/or added to conform
  with this policy.
