package library

import (
	"errors"
	"strconv"
)

const LoadingRetrySeconds = 5

var ErrLoading = errors.New("Library scan is in progress")

func MarkLoadingRetry(header interface{ Set(key, value string) }, err error) bool {
	if !errors.Is(err, ErrLoading) {
		return false
	}
	header.Set("Retry-After", strconv.Itoa(LoadingRetrySeconds))
	header.Set("Cache-Control", "no-store")
	return true
}
