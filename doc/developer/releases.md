---
stage: Systems
group: Distribution
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Versioning

The GitLab Operator uses [semver versioning](https://semver.org/). Version tags should
[be the semver version string](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/adr/0009-version-tagging.md).

## Documentation

Operator documentation is available in the `doc/` directory.

## Red Hat Certification

The release pipeline will contain a `certification_upload` job when the
repository has been tagged with a semver version (i.e. `1.0.0`). This job
will trigger the Red Hat API to request the image be passed through
Red Hat's certification pipeline. The results of the certification pipeline
are published through Red Hat's Connect portal.

It is also possible to pass a release candidate tag (i.e. `1.0.0-rc1`) or a
beta tag (i.e. `1.0.0-beta1`) to trigger the `certification_upload` job.
This will allow the image to go through the Red Hat certification tests, but
will not release the images through the production channel (when that
functionality has been implemented).

It is also possible to add the `certification_upload` job to any pipeline
by setting the CI variable `REDHAT_CERTIFICATION` to the value "true".

In addition, it is possible to run the `scripts/redhat_certification.rb`
script and query the Red Hat API for the status of scan requests that have
been submitted. Executing `scripts/redhat_certification.rb -s` will display
a list of images and their current status in the Red Hat certification
pipeline.

In order to execute the script independently from GitLab CI one needs to
create the `REDHAT_API_TOKEN` environmental variable. This variable is set
to the personal token generated on the [Connect portal](https://connect.redhat.com/account/api-keys).
The token used by GitLab CI is stored in the 1Password Build vault under the
"Red HatCertification Token" entry.

## Retagging a release

When a release pipeline fails or other fixes need to be merged before
a release can be published, the tag needs to be re-created.

This is done by:

1. Merge the required fixes into the stable branch.
1. Delete the tag in [canonical](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator).
1. Delete the tag in the [security fork](https://gitlab.com/gitlab-org/security/cloud-native/gitlab-operator).
1. Delete the tag in the [dev fork](https://dev.gitlab.org/gitlab/cloud-native/gitlab-operator).
1. Confirm the `CHART_VERSIONS` are up to date.
1. Confirm the `appVersion` and `version` in `deploy/chart/Chart.yaml` is up to date.
1. Create the tag again on the HEAD of the stable branch.

   Important: The tag **must** have a description of the following format: `Version a.b.c - supports GitLab Charts x, y, z`
   Without this description the tag is not considered by release tools on the next
   regular release.

1. Confirm the tag pipeline passes.
1. Confirm the tag is mirrored to dev.
