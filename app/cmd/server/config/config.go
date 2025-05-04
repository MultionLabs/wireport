package config

import (
	"os"

	"wireport/internal/logger"

	"github.com/joho/godotenv"
)

func init() {
	envFiles := []string{
		".env",
	}

	for _, envFile := range envFiles {
		if err := godotenv.Load(envFile); err != nil {
			if !os.IsNotExist(err) {
				logger.Warn("Error loading %s: %v", envFile, err)
			}
		}
	}
}

func GetEnv(key string, defaultValue string) string {
	value := os.Getenv(key)

	if value == "" {
		return defaultValue
	}

	return value
}

type Configuration struct {
	ControlServerPort uint16
	DatabasePath      string
	WGPublicPort      uint16

	WireguardConfigPath string
	ResolvConfigPath    string
	DNSMasqConfigPath   string
	CaddyConfigPath     string

	ResolvConfigTemplatePath  string
	DNSMasqConfigTemplatePath string
	CaddyConfigTemplatePath   string

	DockerNetworkName   string
	DockerNetworkDriver string

	WireguardRestartCommand string
	DNSMasqRestartCommand   string
	CaddyRestartCommand     string
}

var DatabasePath = GetEnv("DATABASE_PATH", "/app/wireport/wireport.db")

var Config *Configuration = &Configuration{
	ControlServerPort: 4060,
	DatabasePath:      DatabasePath,
	WGPublicPort:      51820,

	WireguardConfigPath: GetEnv("WIREGUARD_CONFIG_PATH", "/etc/wireguard/wg0.conf"),
	ResolvConfigPath:    GetEnv("RESOLV_CONFIG_PATH", "/etc/resolv.conf"),
	DNSMasqConfigPath:   GetEnv("DNSMASQ_CONFIG_PATH", "/etc/dnsmasq.conf"),
	CaddyConfigPath:     GetEnv("CADDY_CONFIG_PATH", "/etc/caddy/Caddyfile"),

	ResolvConfigTemplatePath:  "configs/resolv/resolv.hbs",
	DNSMasqConfigTemplatePath: "configs/dnsmasq/dnsmasq.hbs",
	CaddyConfigTemplatePath:   "configs/caddy/caddyfile.hbs",

	DockerNetworkName:   "wgp-net",
	DockerNetworkDriver: "bridge",

	WireguardRestartCommand: "/usr/bin/wg-quick down wg0 && /usr/bin/wg-quick up wg0",
	DNSMasqRestartCommand:   "/bin/kill -HUP $(pidof dnsmasq)",
	CaddyRestartCommand:     "/usr/bin/caddy reload --config %s --adapter caddyfile",
}
