// Package controllers reconciles the apps.gitlab.com/v1beta1 GitLab resource.
//
// The package is deprecated and frozen: it serves the v1beta1 resources only,
// and it renders through the equally deprecated helm/ package. Fix bugs here,
// but add nothing new.
//
// The v2 controllers live under internal/controller, one package per resource,
// and render with internal/render. See doc/developer/render.md.
package controllers
