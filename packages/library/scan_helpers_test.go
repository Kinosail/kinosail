package library

import "context"

func ScanContext(ctx context.Context, root, namespace string) ([]Item, error) {
	return NewScanner(root, namespace).Scan(ctx)
}
