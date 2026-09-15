package trustedhttps

// NewTunnelCertificate renews HTTPS through DNS-01 for a private tunnel address.
// It never publishes that address or requires a public TCP listener. Tunnel DNS
// supplies the private address; the existing public DNS record stays unchanged.
func NewTunnelCertificate(domain, token, address, directory string) (*Manager, error) {
	config, err := NewProviderConfig(ProviderDuckDNS, domain, token, address, true)
	if err != nil {
		return nil, err
	}
	manager, err := New(config, directory)
	if err != nil {
		return nil, err
	}
	manager.certificateOnly = true
	return manager, nil
}
