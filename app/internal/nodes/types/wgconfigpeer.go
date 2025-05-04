package types

// WGConfigPeer represents a peer in the WireGuard configuration
type WGConfigPeer struct {
	PublicKey           string            `json:"public_key"`
	Endpoint            *UDPAddrMarshable `json:"endpoint"`
	AllowedIPs          []IPNetMarshable  `json:"allowed_ips"`
	PersistentKeepalive *int              `json:"persistent_keepalive"`
}
