package types

// WGConfig represents the complete WireGuard config
type WGConfig struct {
	Interface WGConfigInterface `json:"interface"`
	Peers     []WGConfigPeer    `json:"peers"`
}
