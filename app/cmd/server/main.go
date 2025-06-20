package main

import (
	"fmt"
	"wireport/cmd/server/commands"
	"wireport/cmd/server/config"
	"wireport/internal/database"
	"wireport/internal/logger"
	"wireport/version"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wireport",
	Short: "VPN tunnel for exposing remote docker services to internet and local network",
	Long: `wireport creates a VPN network that securely exposes remote docker services (by their container names) to both internet and local development environment.
wireport has nodes of three types:

- HOST: a VPS with a public IP address, accessible from the internet
- SERVER: a server to run docker-based workloads on, that should be exposed to both the public internet and local development network (by their container names)
- CLIENT: a developer machine that connects to the wireport network and has access to the docker-based services from the remote server

Spin up a complete wireport setup in four steps:

1. Start the HOST, on a VPS with a public IP address execute the following command:

wireport host start

2. Create a new join-request, on the HOST machine inside the wireport docker execute the following command:

wireport server new

-- the command will output a join-request token; copy the token

3. Start the SERVER, on a machine that should run docker-based workloads execute the following command:

wireport join <TOKEN>

-- here use the token you copied in the previous step

4. Create a new CLIENT wireguard configuration; on the HOST machine inside the wireport docker execute the following command:

wireport client new

-- the command will output a wireguard configuration; copy the configuration and use it on your client machine

Done!
Now you can access the docker-based services from the remote server from your client machine and over the public internet`,
	Version: fmt.Sprintf("%s (commit: %s, date: %s, arch: %s, os: %s)", version.Version, version.Commit, version.Date, version.Arch, version.OS),
}

func main() {
	db, err := database.InitDB()

	if err != nil {
		logger.Fatal("Failed to initialize database at %s: %v", config.Config.DatabasePath, err)
	}

	commands.RegisterCommands(rootCmd, db)

	if err := rootCmd.Execute(); err != nil {
		logger.Error("%v", err)
	}

	defer database.CloseDB(db)
}
