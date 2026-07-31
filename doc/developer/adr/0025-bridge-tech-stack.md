---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 25. Operator Bridge tech stack
---

Date: 2026-07-06

## Status

Accepted

## Context

Bridge is a new component that works alongside GitLab Operator, providing a user interface
through which administrators can interactively manage the operational lifecycle of GitLab and
its components. See [ADR 23](0023-operator-bridge.md) for the overall Bridge architecture.

We need to select the technologies on which to build Bridge.

## Decision

Bridge is built on top of GitLab Operator and comprises a Vue-based frontend and a Go backend.
Go was selected for the backend to align with the existing GitLab Operator codebase, which is
written in Go. Vue was selected for the frontend to leverage the established UI/UX expertise
within the company.

For the frontend, we evaluated pure server-side rendering as an alternative, which would have
reduced the tooling overhead and some of the client-side complexity that a single-page
application introduces. We opted for the single-page application nonetheless, to keep the
backend and frontend loosely coupled and to allow us to draw on in-house frontend expertise as
needed.

The loose coupling also enables the frontend to be supplemented with additional clients in the
future, such as a command-line interface.

## Consequences

- Reusing Go for the backend keeps Bridge aligned with the existing GitLab Operator codebase
  and enables tooling and expertise to be shared across both.
- Adopting Vue for the frontend leverages the established UI/UX expertise within the company.
- The single-page application contract between the frontend and the backend keeps the two
  loosely coupled and allows additional clients, such as a command-line interface, to be added
  in the future.
- The single-page application introduces additional tooling and build complexity that pure
  server-side rendering would have avoided.
