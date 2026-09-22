<!-- After merging changes to this template, update the `Default description template for merge requests` -->
<!-- found under Settings - General Merge Requests -->
## What does this MR do?

<!-- Briefly describe what this MR is about. -->

%{first_multiline_commit}

## Checklist

See [Definition of done](https://gitlab.com/gitlab-org/omnibus-gitlab/blob/master/CONTRIBUTING.md#definition-of-done).

For anything in this list which will not be completed, please provide a reason in the MR discussion.

### Required
- [ ] Merge Request Title and Description are up to date, accurate, and descriptive
- [ ] MR targeting the appropriate branch
- [ ] MR has a green pipeline on GitLab.com
- [ ] When ready for review, MR is marked as ready (not a draft) and assigned to a reviewer.

### Expected (please provide an explanation if not completing)
- [ ] Test plan indicating conditions for success has been posted and passes
- [ ] Documentation created/updated
- [ ] Tests added
- [ ] Integration tests added to [GitLab QA](https://gitlab.com/gitlab-org/gitlab-qa)
- [ ] Equivalent MR/issue for [omnibus-gitlab](https://gitlab.com/gitlab-org/omnibus-gitlab) opened
- [ ] Validate potential values for new configuration settings. Formats such as integer `10`, duration `10s`, URI `scheme://user:passwd@host:port` may require quotation or other special handling when rendered in a template and written to a configuration file.

## Reviewers checklist

- [ ] As soon as you begin reviewing, select **Start a review** on your first comment so the MR shows *Reviewer started review*.

## Related issues

<!-- Link related issues below. Insert the issue link or reference after the word "Closes" if merging this should automatically close it. -->

