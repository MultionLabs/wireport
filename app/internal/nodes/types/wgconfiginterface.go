package types

// WGConfigInterface represents the interface configuration in WireGuard config
type WGConfigInterface struct {
	Address    IPNetMarshable   `json:"address"`
	ListenPort *uint16          `json:"listen_port"`
	PrivateKey string           `json:"private_key"`
	DNS        []IPNetMarshable `json:"dns"`
	PostUp     string           `json:"post_up"`
	PostDown   string           `json:"post_down"`
}
