package types

import (
	"net"
	"strings"
	"testing"

	"wireport/internal/publicservices"
)

func removeSpaces(s string) string {
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func TestNode_GetFormattedCaddyConfig(t *testing.T) {
	wgPort := uint16(51820)
	wgPrivateKey := "1234567890"
	wgPublicKey := "1234567890"

	node := Node{
		ID:            "1",
		Role:          NodeRoleGateway,
		IsCurrentNode: true,
		WGPrivateKey:  "1234567890",
		WGPublicKey:   "1234567890",
		WGConfig: WGConfig{
			Interface: WGConfigInterface{
				ListenPort: &wgPort,
				PrivateKey: wgPrivateKey,
				DNS: []IPNetMarshable{
					{
						IPNet: net.IPNet{
							IP:   net.ParseIP("10.0.0.0"),
							Mask: net.CIDRMask(8, 32),
						},
					},
				},
			},
			Peers: []WGConfigPeer{
				{
					PublicKey: wgPublicKey,
					AllowedIPs: []IPNetMarshable{
						{
							IPNet: net.IPNet{
								IP:   net.ParseIP("10.0.0.0"),
								Mask: net.CIDRMask(8, 32),
							},
						},
					},
				},
			},
		},
	}

	publicServices := []*publicservices.PublicService{
		{
			LocalProtocol:  "http",
			LocalHost:      "localhost",
			LocalPort:      8080,
			PublicProtocol: "http",
			PublicHost:     "localhost",
			PublicPort:     8080,
		},
		{
			LocalProtocol:  "tcp",
			LocalHost:      "192.168.1.100",
			LocalPort:      8080,
			PublicProtocol: "tcp",
			PublicHost:     "127.0.0.1",
			PublicPort:     8080,
		},
	}

	caddyConfig, err := node.GetFormattedCaddyConfig(publicServices)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if caddyConfig == nil {
		t.Errorf("expected caddy config, got nil")
		return
	}

	expectedCaddyConfigRaw := `# layer 4

{
    layer4 {
        # more: https://github.com/mholt/caddy-l4

        tcp/127.0.0.1:8080 {
            route {
                proxy {
                    upstream tcp/192.168.1.100:8080
                }
            }
        }
    }
}

# layer 7

localhost:8080 {
    reverse_proxy http://localhost:8080
}
`

	expectedCaddyConfigNoSpaces := removeSpaces(expectedCaddyConfigRaw)
	caddyConfigNoSpaces := removeSpaces(*caddyConfig)

	if caddyConfigNoSpaces != expectedCaddyConfigNoSpaces {
		t.Errorf(`incorrect caddy config,
got:
%s
expected:
%s`, *caddyConfig, expectedCaddyConfigRaw)
	}
}
