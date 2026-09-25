---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Specs
---

Each spec in this directory states a current product requirement of the Operator. For when to
write or update one, see [Development workflow](../developer/workflow.md).

## Spec rules

- State what the Operator does and why. Leave out implementation details, directions, and
  technical choices. Those belong in an [ADR](../developer/adr/0001-record-architecture-decisions.md).
- Keep the spec short enough for a developer who knows the project to read in a few minutes.
  Aim for about 20 lines before the FAQ.
- Write each requirement as behavior that a test can observe.
- Split a large capability into a parent spec and child specs. The parent links each child, and
  each child links the parent.
- Set `Status` to `Proposed` until the capability ships, then to `Delivered`.

## Spec format

Name the file after the capability, in `snake_case.md`. For a capability with child specs, create a
directory and make the parent spec its `_index.md`.

````markdown
---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: <Capability>
---

Status: Proposed
Issue: <issue link>
Parent: <parent spec link, or none>

## Goal

<Who needs what, and why. One or two sentences.>

## Requirements

- <One observable behavior per line.>

## Out of scope

- <What this spec does not cover.>

## FAQ

- **<Question asked while refining the spec.>** <Answer.>
````
