package commands

import (
	network_apps "wireport/internal/network-apps"

	"github.com/spf13/cobra"
)

var ClientCmd = &cobra.Command{
	Use:   "client",
	Short: "wireport client commands",
	Long:  `Manage wireport client nodes: create a wireguard configuration for the client`,
}

var NewClientCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new wireguard configuration for the client",
	Long:  `Create a new wireguard configuration for the client. The configuration can be used on your client machine to connect to the wireport network`,
	Run: func(cmd *cobra.Command, args []string) {
		clientNode, err := nodes_repository.CreateClient()

		if err != nil {
			cmd.PrintErrf("Failed to create client: %v\n", err)
			return
		}

		hostNode, err := nodes_repository.GetHostNode()

		if err != nil || hostNode == nil {
			cmd.PrintErrf("Failed to get host: %v\n", err)
			return
		}

		publicServices := public_services_repository.GetAll()

		err = hostNode.SaveConfigs(publicServices, false)

		if err != nil {
			cmd.PrintErrf("Failed to save host configs: %v\n", err)
			return
		}

		err = network_apps.RestartNetworkApps(true, false, false)

		if err != nil {
			cmd.PrintErrf("Failed to restart services: %v\n", err)
		}

		wireguardConfig, _ := clientNode.GetFormattedWireguardConfig()

		cmd.Printf("New client created, use the following wireguard config on your client node to connect to the network:\n\n%s\n", *wireguardConfig)
	},
}

func init() {
	ClientCmd.AddCommand(NewClientCmd)
}
