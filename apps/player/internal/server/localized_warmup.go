package server

import (
	"context"
	"errors"
)

const fallbackLanguage = "en"

func WarmLocalizedViews(ctx context.Context) error {
	var failures []error
	for _, view := range registeredLocalizedTemplates() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := view.Prepare(fallbackLanguage); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
