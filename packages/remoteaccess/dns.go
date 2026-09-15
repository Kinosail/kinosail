package remoteaccess

import "context"

// UpdateDNS refreshes an explicitly configured DuckDNS endpoint without opening
// public HTTPS. Private management can therefore outlive the public kill switch.
func UpdateDNS(ctx context.Context, domain, token string) error {
	manager, err := New(Config{Enabled: true, Domain: domain, Token: token})
	if err != nil {
		return err
	}
	return manager.update(ctx)
}
