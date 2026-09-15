package server

import (
	"context"
	"errors"
	"fmt"
)

type authRelatedDocument struct {
	name string
	path string
}

func (auth *authentication) persistSettingsAnd(ctx context.Context, next installationSettings, related authRelatedDocument, value any) error {
	if auth.profiles.database != nil {
		return auth.profiles.database.SaveJSONBatchContext(ctx, map[string]any{
			"settings.json": next,
			related.name:    value,
		})
	}
	previous := auth.settings.value
	if err := auth.settings.save(next); err != nil {
		return err
	}
	if err := auth.profiles.persist(related.path, value); err != nil {
		if rollbackErr := auth.settings.save(previous); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore settings: %w", rollbackErr))
		}
		return err
	}
	return nil
}
