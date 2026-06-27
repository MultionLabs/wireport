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

	PublishDockerSocketScriptTemplatePath   string
	UnpublishDockerSocketScriptTemplatePath string

	RunitServiceDir          string
	RunitServiceDisabledDir  string
	DockerSocketServiceName  string
	DockerSocketTCPPort      string
	DockerSocketUnixPath     string

	DockerNetworkName   string
	DockerNetworkDriver string

	WireguardRestartCommand          string
	CaddyRestartCommand              string
	CoreDNSRestartCommand            string
	ServerJoinVerificationCommandFmt string

	DocumentationURL string

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

	PublishDockerSocketScriptTemplatePath:   "scripts/publish/docker-socket.hbs",
	UnpublishDockerSocketScriptTemplatePath: "scripts/unpublish/docker-socket.hbs",

	RunitServiceDir:          "/etc/service",
	RunitServiceDisabledDir:  "/etc/service-disabled",
	DockerSocketServiceName:  "socat-docker-socket",
	DockerSocketTCPPort:      "2375",
	DockerSocketUnixPath:     "/var/run/docker.sock",

	DockerNetworkName:   "wireport-net",
	DockerNetworkDriver: "bridge",

	// || true on down: wg0 may not exist yet right after join (runit starts wireguard separately). Real wg-quick up failures still fail the command
	WireguardRestartCommand: "(/usr/bin/wg-quick down wg0 2>/dev/null || true) && /usr/bin/wg-quick up wg0",
	CaddyRestartCommand:     "/usr/bin/caddy reload --config %s --adapter caddyfile",
	// || true when coredns is not running: expected before runit execs CoreDNS. TERM vs kill -9: gentler; node may schedule another restart
	CoreDNSRestartCommand:            "/usr/bin/pkill -TERM coredns 2>/dev/null || true",
	ServerJoinVerificationCommandFmt: "docker exec %s sh -c 'test -f %s && test -f %s'",

	DocumentationURL: GetEnv("WIREPORT_DOCUMENTATION_URL", "https://github.com/MultionLabs/wireport"),

	WireportGatewayContainerName:  "wireport-gateway",
	WireportGatewayContainerImage: "ghcr.io/multionlabs/wireport",
	WireportServerContainerName:   "wireport-server",
	WireportServerContainerImage:  "ghcr.io/multionlabs/wireport",

	CertExpiry: time.Hour * 24 * 365 * 5, // 5 years
}
