package remoteaccess

import "context"

func UpdateDNS(ctx context.Context, domain, token string) error {
	manager, err := New(Config{Enabled: true, Domain: domain, Token: token})
	if err != nil {
		return err
	}
	return manager.update(ctx)
}
