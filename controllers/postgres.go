package controllers

import (
	"context"

	"gitlab.com/gitlab-org/cloud-native/gitlab-operator/pkg/gitlab"
)

func (r *GitLabReconciler) validateExternalPostgresConfiguration(ctx context.Context, adapter gitlab.Adapter) error {
	// Ensure that the PostgreSQL password Secret was created.
	pgSecretName := adapter.Values().GetString("global.psql.password.secret")
	if err := r.ensureSecret(ctx, adapter, pgSecretName); err != nil {
		return err
	}

	// If set, ensure that the PostgreSQL SSL Secret was created.
	pgSecretNameSSL := adapter.Values().GetString("global.psql.ssl.secret", "unset")
	if pgSecretNameSSL != "unset" {
		if err := r.ensureSecret(ctx, adapter, pgSecretNameSSL); err != nil {
			return err
		}
	}

	return nil
}
