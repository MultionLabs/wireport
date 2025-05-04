package types

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/logger"
	templates "wireport/internal/templates"

	"github.com/aymerick/raymond"
)

func init() {
	raymond.RegisterHelper("split", func(input string, separator string) []string {
		return strings.Split(input, separator)
	})

	raymond.RegisterHelper("ipsToStrings", func(ipnets []IPNetMarshable, includeMask bool) []string {
		return MapIPNetMarshablesToStrings(ipnets, includeMask)
	})

	raymond.RegisterHelper("ipToString", func(ipnet IPNetMarshable) string {
		return IPToString(ipnet.IP)
	})

	raymond.RegisterHelper("splitAndTakeN", func(input string, separator string, n int) string {
		parts := strings.Split(input, separator)

		if len(parts) < n {
			return input
		}

		return parts[n-1]
	})
}

type NodeRole string

const (
	NodeRoleHost   NodeRole = "host"
	NodeRoleClient NodeRole = "client"
	NodeRoleServer NodeRole = "server"
)

// Node represents a virtual machine in the system (host, client, server)
type Node struct {
	ID   string   `gorm:"type:text;primary_key"`
	Role NodeRole `gorm:"type:text;not null"`

	WGPrivateKey string `gorm:"type:text;not null"`
	WGPublicKey  string `gorm:"type:text;not null"`

	WGConfig WGConfig `gorm:"type:text;serializer:json;uniqueIndex;not null"`

	WGPublicIp   *string `gorm:"type:text"`
	WGPublicPort *uint16 `gorm:"type:integer"`

	ConnectionEncryptionKey *string `gorm:"type:text"`

	DockerSubnet *IPNetMarshable `gorm:"type:text;serializer:json"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (c *WGConfig) ToINI() (*string, error) {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")

	sb.WriteString(fmt.Sprintf("Address = %s\n", c.Interface.Address.String()))

	if c.Interface.ListenPort != nil {
		sb.WriteString(fmt.Sprintf("ListenPort = %d\n", *c.Interface.ListenPort))
	}

	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", c.Interface.PrivateKey))

	if len(c.Interface.DNS) > 0 {
		dnsStrings := MapIPNetMarshablesToStrings(c.Interface.DNS, false)
		sb.WriteString(fmt.Sprintf("DNS = %s\n", strings.Join(dnsStrings, ", ")))
	}

	if c.Interface.PostUp != "" {
		sb.WriteString(fmt.Sprintf("PostUp = %s\n", c.Interface.PostUp))
	}

	if c.Interface.PostDown != "" {
		sb.WriteString(fmt.Sprintf("PostDown = %s\n", c.Interface.PostDown))
	}

	for _, peer := range c.Peers {
		sb.WriteString("\n[Peer]\n")

		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))

		if peer.Endpoint != nil && peer.Endpoint.String() != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint.String()))
		}

		if len(peer.AllowedIPs) > 0 {
			allowedIPsStrings := MapIPNetMarshablesToStrings(peer.AllowedIPs, true)
			sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(allowedIPsStrings, ", ")))
		}

		if peer.PersistentKeepalive != nil {
			sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", *peer.PersistentKeepalive))
		}
	}

	result := sb.String()
	return &result, nil
}

func (n *Node) GetFormattedWireguardConfig() (*string, error) {
	output, err := n.WGConfig.ToINI()

	if err != nil {
		return nil, err
	}

	return output, nil
}

func (n *Node) GetFormattedDNSMasqConfig() (*string, error) {
	if n.Role == NodeRoleClient {
		return nil, errors.New("client nodes do not need a DNSMasq config")
	}

	template, err := templates.Configs.ReadFile(config.Config.DNSMasqConfigTemplatePath)

	if err != nil {
		return nil, err
	}

	tpl, err := raymond.Parse(string(template))

	if err != nil {
		return nil, err
	}

	configContents, err := tpl.Exec(n)

	if err != nil {
		return nil, err
	}

	return &configContents, nil
}

func (n *Node) GetFormattedResolvConfig() (*string, error) {
	if n.Role != NodeRoleHost {
		return nil, errors.New("only host nodes can have a resolv config")
	}

	template, err := templates.Configs.ReadFile(config.Config.ResolvConfigTemplatePath)

	if err != nil {
		return nil, err
	}

	tpl, err := raymond.Parse(string(template))

	if err != nil {
		return nil, err
	}

	configContents, err := tpl.Exec(n)

	if err != nil {
		return nil, err
	}

	return &configContents, nil
}

func (n *Node) GetFormattedCaddyConfig() (*string, error) {
	if n.Role != NodeRoleHost {
		return nil, errors.New("only host nodes can have a Caddy config")
	}

	template, err := templates.Configs.ReadFile(config.Config.CaddyConfigTemplatePath)

	if err != nil {
		return nil, err
	}

	tpl, err := raymond.Parse(string(template))

	if err != nil {
		return nil, err
	}

	configContents, err := tpl.Exec(n)

	if err != nil {
		return nil, err
	}

	return &configContents, nil
}

func (n *Node) SaveConfigs() error {
	if n.Role != NodeRoleHost && n.Role != NodeRoleServer {
		return errors.New("config saving is only relevant to host and server nodes")
	}

	wireguardConfig, _ := n.GetFormattedWireguardConfig()

	dnsMasqConfig, _ := n.GetFormattedDNSMasqConfig()

	resolvConfig, _ := n.GetFormattedResolvConfig()

	caddyConfig, _ := n.GetFormattedCaddyConfig()

	if dnsMasqConfig != nil {
		// update is needed for both host and server nodes

		logger.Info("Writing dnsmasq config to %s", config.Config.DNSMasqConfigPath)
		err := os.WriteFile(config.Config.DNSMasqConfigPath, []byte(*dnsMasqConfig), 0644)

		if err != nil {
			logger.Fatal("Failed to get formatted dnsmasq config: %v", err)
			return err
		}
	}

	if n.Role == NodeRoleHost {
		if wireguardConfig != nil {
			logger.Info("Writing wireguard config to %s", config.Config.WireguardConfigPath)
			err := os.WriteFile(config.Config.WireguardConfigPath, []byte(*wireguardConfig), 0644)

			if err != nil {
				logger.Fatal("Failed to get formatted wireguard config: %v", err)
				return err
			}
		}

		if resolvConfig != nil {
			logger.Info("Writing resolv config to %s", config.Config.ResolvConfigPath)
			err := os.WriteFile(config.Config.ResolvConfigPath, []byte(*resolvConfig), 0644)

			if err != nil {
				logger.Fatal("Failed to get formatted resolv config: %v", err)
				return err
			}
		}

		if caddyConfig != nil {
			logger.Info("Writing caddy config to %s", config.Config.CaddyConfigPath)
			err := os.WriteFile(config.Config.CaddyConfigPath, []byte(*caddyConfig), 0644)

			if err != nil {
				logger.Fatal("Failed to get formatted caddy config: %v", err)
				return err
			}
		}
	}

	return nil
}
