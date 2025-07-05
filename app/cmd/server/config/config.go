package config

import (
	"os"
	"path/filepath"
	"time"

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

func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Warn("Could not determine home directory: %v", err)
		return ""
	}
	return homeDir
}

func getDefaultDatabasePath(fallback string, profile string) string {
	homeDir := getHomeDir()
	if homeDir == "" {
		return fallback
	}
	return filepath.Join(homeDir, ".wireport", profile, "wireport.db")
}

type Configuration struct {
	ControlServerPort uint16
	DatabasePath      string
	WGPublicPort      uint16

	WireportProfile string

	WireguardConfigPath string
	ResolvConfigPath    string
	CaddyConfigPath     string
	CoreDNSConfigPath   string

	ResolvConfigTemplatePath  string
	CaddyConfigTemplatePath   string
	CoreDNSConfigTemplatePath string

	UpGatewayScriptTemplatePath      string
	UpServerScriptTemplatePath       string
	DownServerScriptTemplatePath     string
	DownGatewayScriptTemplatePath    string
	UpgradeGatewayScriptTemplatePath string
	UpgradeServerScriptTemplatePath  string
	NewClientScriptTemplatePath      string

	DockerNetworkName   string
	DockerNetworkDriver string

	WireguardRestartCommand string
	CaddyRestartCommand     string
	CoreDNSRestartCommand   string

	WireportGatewayContainerName  string
	WireportGatewayContainerImage string
	WireportServerContainerName   string
	WireportServerContainerImage  string

	CertExpiry time.Duration
}

var WireportProfile = GetEnv("WIREPORT_PROFILE", "default")
var DatabasePath = GetEnv("DATABASE_PATH", getDefaultDatabasePath("/app/wireport/wireport.db", WireportProfile))

var Config = &Configuration{
	ControlServerPort: 4060,
	DatabasePath:      DatabasePath,
	WGPublicPort:      51820,

	WireportProfile: WireportProfile,

	ResolvConfigPath:    GetEnv("RESOLV_CONFIG_PATH", "/etc/resolv.conf"),
	WireguardConfigPath: GetEnv("WIREGUARD_CONFIG_PATH", "/etc/wireguard/wg0.conf"),
	CaddyConfigPath:     GetEnv("CADDY_CONFIG_PATH", "/etc/caddy/Caddyfile"),
	CoreDNSConfigPath:   GetEnv("COREDNS_CONFIG_PATH", "/etc/coredns/Corefile"),

	ResolvConfigTemplatePath:  "configs/resolv/resolv.hbs",
	CaddyConfigTemplatePath:   "configs/caddy/caddyfile.hbs",
	CoreDNSConfigTemplatePath: "configs/coredns/corefile.hbs",

	UpGatewayScriptTemplatePath:      "scripts/up/gateway.hbs",
	UpServerScriptTemplatePath:       "scripts/up/server.hbs",
	DownGatewayScriptTemplatePath:    "scripts/down/gateway.hbs",
	DownServerScriptTemplatePath:     "scripts/down/server.hbs",
	UpgradeGatewayScriptTemplatePath: "scripts/upgrade/gateway.hbs",
	UpgradeServerScriptTemplatePath:  "scripts/upgrade/server.hbs",
	NewClientScriptTemplatePath:      "scripts/new/client.hbs",

	DockerNetworkName:   "wireport-net",
	DockerNetworkDriver: "bridge",

	WireguardRestartCommand: "/usr/bin/wg-quick down wg0 && /usr/bin/wg-quick up wg0",
	CaddyRestartCommand:     "/usr/bin/caddy reload --config %s --adapter caddyfile",
	CoreDNSRestartCommand:   "/bin/kill -9 $(pidof coredns)", // with actual restart (not -HUP) - to drop the cache

	WireportGatewayContainerName:  "wireport-gateway",
	WireportGatewayContainerImage: "anybotsllc/wireport",
	WireportServerContainerName:   "wireport-server",
	WireportServerContainerImage:  "anybotsllc/wireport",

	CertExpiry: time.Hour * 24 * 365 * 5, // 5 years
}
