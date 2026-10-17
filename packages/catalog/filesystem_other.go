//go:build !linux

package catalog

func networkFilesystem(string) bool { return false }

func nestedNetworkMounts([]ScanRoot) map[ScanRoot][]string { return nil }
