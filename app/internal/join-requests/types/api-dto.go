package types

type JoinConfigs struct {
	WireguardConfig *string `json:"wireguardConfig"`
	ResolvConfig    *string `json:"resolvConfig"`
	CoreDNSConfig   *string `json:"coreDNSConfig"`
	DockerSubnet    *string `json:"dockerSubnet"`
}

type JoinRequestDTO struct {
	JoinToken string `json:"joinToken"`
}

type JoinResponseDTO struct {
	JoinConfigs *JoinConfigs `json:"joinConfigs"`
}
