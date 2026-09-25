---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Development workflow
---

Follow these steps for every change. Agents follow them too, through [`AGENTS.md`](../../AGENTS.md).

## Write the spec

- Write a spec for each new or changed capability before you implement it. Follow
  [Specs](../specs/_index.md). Bug fixes and maintenance work need no spec.
- Question the requester until the requirements are unambiguous. Record each question and answer
  in the FAQ section of the spec.
- Update the spec in the merge request that changes the behavior. Delete the spec in the merge
  request that removes the capability.

## Record technical decisions

A spec says what the Operator does. An ADR records how, and why that way.

- Add an ADR to [`doc/developer/adr/`](adr/0001-record-architecture-decisions.md) as
  `NNNN-short-title.md`, with Context, Decision, and Consequences sections.
- Keep the ADR readable in under 10 minutes. Link the spec instead of restating it.
- Skip the ADR when the change follows an existing ADR or convention.

## Coordinate with issues

- Open an issue for work that spans more than one merge request. Link the parent spec from it.
- State the problem and possible directions in the issue. Leave the plan to whoever takes it.
- Record caveats and deferred work as backlog issues. Link them from the merge request.

## Test what matters

Aim for meaningful coverage, not a high test count. For the test tiers, see [Tests](testing.md).

- Cover each spec requirement with at least one unit or end-to-end test.
- Test what a user or caller observes. Do not test internals, generated code, or trivial accessors.
- Use unit tests for rendering and mapping logic, and end-to-end tests for behavior in a cluster.
- Start each bug fix with a test that fails on the current code.
- Do not add a test or suite that always skips.

## Prove the delivery

Do not claim a change is done without proof. Show one of:

- An automated test: name the test, give the command, and show that it passes.
- A manual test: list the steps and commands, and include the observed output.

Put the proof in the merge request description. Documentation-only changes and small fixes or
improvements are exempt. State the exemption instead.

## Review against the spec

Before you mark a merge request as ready, have a fresh sub-agent or session review it. The reviewer
must not share context with the implementation.

1. Give the reviewer the spec, the ADR if any, and the diff.
1. Ask whether the change delivers every requirement, and what it misses or adds beyond the spec.
1. Fix the gaps, or record them as backlog issues.

## Label issues and merge requests

Label every issue and merge request you create. See [Labels](labels.md).

## Keep the GitLab Duo review instructions current

GitLab Duo Code Review does not read `AGENTS.md`. It checks the rules on this page and in
[Specs](../specs/_index.md) through
[`.gitlab/duo/mr-review-instructions.yaml`](../../.gitlab/duo/mr-review-instructions.yaml).
When you change one of these rules, update that file in the same merge request.
