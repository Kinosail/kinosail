package trustedhttps

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
