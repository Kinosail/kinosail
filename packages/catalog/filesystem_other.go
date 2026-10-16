//go:build !linux

package catalog

func networkFilesystem(string) bool { return false }
