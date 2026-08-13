// Package helm renders the Helm chart of the apps.gitlab.com/v1beta1 GitLab
// resource through a client-side dry run of a Helm install, and queries the
// rendered template.
//
// The package is deprecated and frozen: it serves the v1beta1 GitLab controller
// and webhook only. Fix bugs here, but add nothing new.
//
// New code renders with internal/render, which templates a chart into
// unstructured objects with no cluster connection and no release machinery. See
// doc/developer/render.md.
package helm
