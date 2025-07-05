package main

import (
	"fmt"
	"wireport/cmd/server/commands"
	"wireport/cmd/server/config"
	"wireport/internal/database"
	"wireport/version"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wireport",
	Short: "Ingress proxy for exposing local services and remote Docker-based workloads to the Internet",
	Long: `wireport is a self-hosted ingress proxy and VPN tunnel that securely exposes private local and Docker-based services to the Internet, with free, automatically renewable SSL certificates. Powered by WireGuard (secure networking), CoreDNS and Caddy (performant reverse proxy).

- Exposing local and Docker-based services running in a local network (e.g., on the local machine, on a corporate network, on a NAS, or on a home server) to the Internet
- Secure tunneling into remote development/staging/production environments to facilitate debugging and troubleshooting of remote Docker-based services.

Key Concepts:

- GATEWAY – a Linux-based machine with Docker installed, a public IP address, and the following open ports: 80/tcp, 443/tcp, 4060/tcp and 51820/udp. This node acts as the ingress gateway and an entry point to your published services.
- CLIENT – any number of laptops/PCs that will connect to the WireGuard network to manage the ingress network and expose services.
- SERVER (optional) – one or more Linux-based machines (with Docker) that run the workloads you want to expose. These nodes join the same private WireGuard network, provided by the GATEWAY.

Spin up a complete wireport setup in just two commands:

1. Bootstrap the GATEWAY, on a VPS with a public IP address execute the following command:

wireport gateway up sshuser@140.120.110.10:22

(replace sshuser with a username that has access to the VPS, 140.120.110.10 with your VPS public IP address, 22 with your SSH port)

2. Expose a local service to the Internet:

wireport service publish \
  --local  http://10.0.0.2:3000 \
  --public https://demo.example.com:443

(replace 10.0.0.2 with the IP address of the local service, demo.example.com with the public domain name of the service)
(check help of 'wireport service publish' for more details on supported protocols and allowed ports)

Done!

Now you should be able to access the Docker-based services from your local machine over the public Internet.

If the installation process fails, or the service is not accessible over the Internet, make sure that:

- the required ports are open on the gateway VPS (80/tcp, 443/tcp, 4060/tcp and 51820/udp)
- there's a correct DNS A-record, pointing to your gateway VPS
- the gateway VPS has docker installed
- the ssh user, used for bootstraping the gateway VPS, is allowed to run docker commands on the gateway VPS

If this does not help, check the logs of the wireport docker container on gateway.
`,
	Version: fmt.Sprintf("%s (commit: %s, date: %s, arch: %s, os: %s, package: %s); db path: %s; profile: %s", version.Version, version.Commit, version.Date, version.Arch, version.OS, version.Package, config.DatabasePath, config.WireportProfile),
}

func main() {
	db, err := database.InitDB()

	if err != nil {
		rootCmd.PrintErrf("Failed to initialize database at %s: %v\n", config.Config.DatabasePath, err)
	}

	commands.RegisterCommands(rootCmd, db)

	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrf("%v\n", err)
	}

	defer func() {
		if err := database.CloseDB(db); err != nil {
			rootCmd.PrintErrf("Failed to close database: %v\n", err)
		}
	}()
}
