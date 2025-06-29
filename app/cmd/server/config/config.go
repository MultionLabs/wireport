package config

import (
	"os"
	"path/filepath"
	"time"

	"wireport/internal/logger"
	"wireport/version"

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

func getDefaultDatabasePath(fallback string) string {
	homeDir := getHomeDir()
	if homeDir == "" {
		return fallback
	}
	return filepath.Join(homeDir, ".wireport", version.Version, "wireport.db")
}

type Configuration struct {
	ControlServerPort uint16
	DatabasePath      string
	WGPublicPort      uint16

	WireguardConfigPath string
	ResolvConfigPath    string
	CaddyConfigPath     string
	CoreDNSConfigPath   string

	ResolvConfigTemplatePath  string
	CaddyConfigTemplatePath   string
	CoreDNSConfigTemplatePath string

	BootstrapHostScriptTemplatePath    string
	NewClientScriptTemplatePath        string
	ConnectServerScriptTemplatePath    string
	DisconnectServerScriptTemplatePath string
	UpgradeHostScriptTemplatePath      string
	UpgradeServerScriptTemplatePath    string

	DockerNetworkName   string
	DockerNetworkDriver string

	WireguardRestartCommand string
	CaddyRestartCommand     string
	CoreDNSRestartCommand   string

	WireportHostContainerName    string
	WireportHostContainerImage   string
	WireportServerContainerName  string
	WireportServerContainerImage string

	CertExpiry time.Duration
}

var DatabasePath = GetEnv("DATABASE_PATH", getDefaultDatabasePath("/app/wireport/wireport.db"))

var Config = &Configuration{
	ControlServerPort: 4060,
	DatabasePath:      DatabasePath,
	WGPublicPort:      51820,

	WireguardConfigPath: GetEnv("WIREGUARD_CONFIG_PATH", "/etc/wireguard/wg0.conf"),
	ResolvConfigPath:    GetEnv("RESOLV_CONFIG_PATH", "/etc/resolv.conf"),
	CaddyConfigPath:     GetEnv("CADDY_CONFIG_PATH", "/etc/caddy/Caddyfile"),
	CoreDNSConfigPath:   GetEnv("COREDNS_CONFIG_PATH", "/etc/coredns/Corefile"),

	ResolvConfigTemplatePath:  "configs/resolv/resolv.hbs",
	CaddyConfigTemplatePath:   "configs/caddy/caddyfile.hbs",
	CoreDNSConfigTemplatePath: "configs/coredns/corefile.hbs",

	BootstrapHostScriptTemplatePath:    "scripts/bootstrap/host.hbs",
	NewClientScriptTemplatePath:        "scripts/new/client.hbs",
	ConnectServerScriptTemplatePath:    "scripts/connect/server.hbs",
	DisconnectServerScriptTemplatePath: "scripts/disconnect/server.hbs",
	UpgradeHostScriptTemplatePath:      "scripts/upgrade/host.hbs",
	UpgradeServerScriptTemplatePath:    "scripts/upgrade/server.hbs",

	DockerNetworkName:   "wireport-net",
	DockerNetworkDriver: "bridge",

	WireguardRestartCommand: "/usr/bin/wg-quick down wg0 && /usr/bin/wg-quick up wg0",
	CaddyRestartCommand:     "/usr/bin/caddy reload --config %s --adapter caddyfile",
	CoreDNSRestartCommand:   "/bin/kill -9 $(pidof coredns)", // with actual restart (not -HUP) - to drop the cache

	WireportHostContainerName:    "wireport-host",
	WireportHostContainerImage:   "anybotsllc/wireport",
	WireportServerContainerName:  "wireport-server",
	WireportServerContainerImage: "anybotsllc/wireport",

	CertExpiry: time.Hour * 24 * 365 * 5, // 5 years
}
