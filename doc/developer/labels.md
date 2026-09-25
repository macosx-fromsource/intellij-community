---
stage: GitLab Delivery
group: Operate
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Labels
---

Label every issue and merge request you create. These rules change often, so check this page each
time. The source of truth for type labels is the
[work type classification](https://handbook.gitlab.com/handbook/product/groups/product-analysis/engineering/metrics/#work-type-classification). You can ask the AI agent to update this page as the rules change.

## Required labels

| Label | Rule |
|---|---|
| `group::operate` | Always. The triage bot adds the matching `devops::` and `section::` labels. |
| `type::feature`, `type::bug`, or `type::maintenance` | Exactly one. |
| Subtype | One that matches the type. See [Choose the subtype](#choose-the-subtype). |

## Choose the type

Classify by the intent of the work, not by the diff.

- `type::feature`: Users gain a capability they did not have, or an existing capability changes.
- `type::bug`: Shipped behavior differs from what was intended or documented.
- `type::maintenance`: The work is neither. For example, refactoring, dependency updates, CI,
  tests only, edits to existing documentation, and releases.

Resolve edge cases this way:

- New documentation for a feature or bug takes the type of that feature or bug.
- A fix to a capability that has not shipped yet is `type::feature`.
- Missing functionality that was in the original acceptance criteria is `type::bug`.
- A spike takes the type of the work it leads to. Add `spike` as well.
- A planning issue that fits no type gets `type::ignore`.
- A merge request that mixes intents should be split.
- If the type is unclear, ask a human instead of guessing.

## Choose the subtype

| Subtype | Example | Use for |
|---|---|---|
| `feature::addition` | Add the Siphon custom resource. | The first version of a capability. |
| `feature::enhancement` | Add a form to the bridge SPA. | A later improvement to an existing capability. |
| `feature::consolidation` | | Merging a capability into another one. |
| `bug::functional` | Fetch the Siphon chart only for the bridge image. | Wrong behavior. Most Operator bugs. |
| `bug::vulnerability` | Fix a CVE in the Operator image. | Security defects. |
| `bug::performance` | | Slow reconciles or excessive resource use. |
| `bug::ux` | | Confusing behavior in the bridge SPA or `kubectl bridge`. |
| `bug::transient` | | Intermittent defects. |
| `maintenance::dependency` | Update `helm.sh/helm/v4`. | Dependency and base image updates. |
| `maintenance::refactor` | Describe the single OpenShift CI cluster. | Restructuring code, or editing existing documentation. |
| `maintenance::test-gap` | Add a black-box end-to-end framework. | Tests only. |
| `maintenance::pipelines` | Add Kubernetes 1.37 testing. | CI configuration. |
| `maintenance::workflow` | Use the native review flow. | Developer tooling, linters, templates, and agent instructions. |
| `maintenance::release` | Backport security fixes to 3.1.x. | Releases and backports. |
| `maintenance::removal` | Drop an unused provisioning script. | Removing a capability or dead code. |
| `maintenance::performance` | | Performance improvements that fix no defect. |
| `maintenance::scalability` | | Scalability changes that users do not see. |
| `maintenance::usability` | | Usability improvements outside planned feature work. |

## Other labels

- `documentation`: Add to merge requests that change documentation.
- `pipeline failure`: Add to issues about a failed pipeline, with
  `pipeline failure::needs investigation`.
- `Operate::BAU`: Add to cross-functional business-as-usual work of the Operate team.
- Leave `priority::`, `severity::`, and the milestone to the human who triages, unless asked.

## Apply labels

Add a quick action to the issue or merge request description:

```plaintext
/label ~"group::operate" ~"type::feature" ~"feature::addition"
```

Or pass the labels to `glab`:

```shell
glab mr create --label "group::operate,type::feature,feature::addition"
```

A merge request usually takes the labels of its linked issue. Label it by its own change when that
differs. For example, a tests-only merge request for a feature issue is `maintenance::test-gap`.
Set the labels before the merge request merges.
