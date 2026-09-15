package wireguard

import "log/slog"

// OpenOptional returns nil when pairing is unconfigured or unavailable.
func OpenOptional(directory, endpoint string) *Manager {
	if directory == "" || endpoint == "" {
		return nil
	}
	manager, err := Open(directory, endpoint)
	if err != nil {
		slog.Warn("Verified Direct Connection unavailable", "error", err)
	}
	return manager
}
