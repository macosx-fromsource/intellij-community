---
stage: GitLab Delivery
group: Self Managed
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Developer Guide
---

This developer guide aims to walk a new developer on how to set up their environment to be able to contribute to this project.

## Setting up development environment

To set up your system for development of the operator, follow the steps below:

1. Clone the `gitlab-operator` repository into your GOPATH.

   ```shell
   git clone git@gitlab.com:gitlab-org/cloud-native/gitlab-operator.git
   cd gitlab-operator
   ```

1. Install [`asdf`](https://asdf-vm.com) to manage runtime dependencies.

1. Install runtime dependencies.

   ```shell
   cut -d' ' -f1 .tool-versions | xargs -i asdf plugin add {}
   asdf plugin add opm https://gitlab.com/dmakovey/asdf-opm.git
   asdf install
   ```

1. Run `task` from the root of the repository to see available commands.

   We use [`task` from taskfile.dev](https://taskfile.dev/) in place of `make` for this project. [Here is why](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/doc/developer/adr/0016-replace-makefile-with-taskfile.md).

   Do not install `task` with `brew install task`. You will end up with [Taskwarrior](https://taskwarrior.org/docs/), which is a different package. Either user the `asdf` plugin from above, or follow [installation instructions from taskfile.dev](https://taskfile.dev/installation/).

   See
   [`Taskfile.yaml`](https://gitlab.com/gitlab-org/cloud-native/gitlab-operator/-/blob/master/Taskfile.yaml?ref_type=heads)
   for our task definitions.

## Project structure

The GitLab Operator is built using the Operator SDK v1.0.0 and consequently uses the Kubebuilder v2 layout format. This is necessary to know since there was a change in project directory and some of the tooling used by operator SDK.

```shell
$ pwd
gitlab-operator
$ tree -dL 2 .
.
├── api
│   └── v1beta1
├── bundle
│   ├── manifests
│   ├── metadata
│   └── tests
├── config
│   ├── certmanager
│   ├── crd
│   ├── default
│   ├── deploy
│   ├── manager
│   ├── manifests
│   ├── prometheus
│   ├── rbac
│   ├── samples
│   ├── scorecard
│   ├── test
│   └── webhook
├── controllers
│   ├── backup
│   ├── gitlab
│   ├── helpers
│   ├── runner
│   ├── settings
│   ├── testdata
│   └── utils
├── doc
├── hack
│   └── assets
├── helm
│   └── testdata
└── scripts
    └── manifests
```

- The `controllers` directory contains the controller implementations for the GitLab and GitLab Backup controllers.
- The `api` directory contains the API resource definitions for the GitLab and GLBackup resources owned by the operator. The API definitions are grouped by their API version.
  The `*_types.go` file inside `api/<api_version>` contains spec definitions and markers used to generate the custom resource definitions and Cluster Service Version file used by OLM.
- The `config/samples` directory contains an example manifest for the GitLab custom resource.
- The `config/test` directory contains a parametrized GitLab definition used for running integration tests.

  An example is shown below:

  `// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete`

  The contents of `config/rbac/custom` were created manually and is not affected by the RBAC markers.

  Most of the other contents of the config directory are automatically generated but could be modified using `kustomize`.

- The `hack/assets` path contains resources that would need to be pushed inside the operator image when the container image is being built. This is where release files would go.

## Deploying the Operator

For instructions on deploying the operator, see the [installation docs](installation.md).

## Debugging

There have been a couple of functions added to `controllers/gitlab/template_test.go`
to assist in the development of features and the writing of tests.

- `dumpTemplate(template)`
- `dumpTemplateToFile(template, filename)`
- `dumpHelmValues(values)`
- `dumpHelmValuesToFile(values, filename)`

The `dumpTemplate()` function will take the template object from the GitLab
adapter and return the rendered YAML of the Helm chart as a string. Since
the Go test framework will absorb anything written to stdout, the
`dumpTemplateToFile()` will write the YAML to a file for inspection. It
is important to note that if just a filename is provided that the file will
be written to the subdirectory where the test file resides rather than the
directory where the tests were initiated from. An absolute file path is
necessary if one desires the file to be written where the tests are
initiated from.

Similarly the `dumpHelmValues()` will return the YAML representation of the
Helm values as string. This is can be used to verify that the intended
values are set at the beginning of any tests. The `dumpHelmValues()` function
is used to write the YAML to a file for inspection and the filename argument
has the same limitations as `dumpTemplateToFile()`.
