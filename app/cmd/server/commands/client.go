package commands

import (
	"net"
	"wireport/cmd/server/config"
	network_apps "wireport/internal/network-apps"
	"wireport/internal/nodes/types"

	"github.com/spf13/cobra"
)

var forceClientCreation bool = false
var quietClientCreation bool = false

var ClientCmd = &cobra.Command{
	Use:   "client",
	Short: "wireport client commands",
	Long:  `Manage wireport client nodes: create a wireguard configuration for the client`,
}

var NewClientCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a client to wireport network",
	Long:  `Create a new join-request for connecting a client to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command)`,
	Run: func(cmd *cobra.Command, args []string) {
		totalWireguardClients, availableWireguardClients, err := nodes_repository.TotalAvailableWireguardClients()

		if err != nil {
			cmd.PrintErrf("Failed to count available wireguard clients: %v\n", err)
			return
		}

		totalJoinRequests := join_requests_repository.CountAll()

		if availableWireguardClients <= 0 || totalJoinRequests >= availableWireguardClients {
			cmd.PrintErrf("No available wireguard client slots. Please delete some client/server nodes (total used: %d) or client/server join-requests (total used: %d) to free up some wireguard client slots.\n", totalWireguardClients, totalJoinRequests)
			return
		}

		hostNode, err := nodes_repository.GetHostNode()

		if err != nil || hostNode == nil {
			cmd.PrintErrf("Failed to get host: %v\n", err)
			return
		}

		if forceClientCreation {
			if !quietClientCreation {
				cmd.Printf("Force flag detected, creating client node without generating a join request\n")
			}

			clientNode, err := nodes_repository.CreateClient()

			if err != nil {
				cmd.PrintErrf("Failed to create client: %v\n", err)
				return
			}

			if !quietClientCreation {
				cmd.Printf("Client node created without join request\n")
			}

			// save configs & restart services
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

			if !quietClientCreation {
				cmd.Printf("New client created, use the following wireguard config on your client node to connect to the network:\n\n%s\n", *wireguardConfig)
			} else {
				cmd.Printf("%s\n", *wireguardConfig)
			}
		} else {
			// create join request
			joinRequest, err := join_requests_repository.Create(types.UDPAddrMarshable{
				UDPAddr: net.UDPAddr{
					IP:   net.ParseIP(*hostNode.WGPublicIp),
					Port: int(config.Config.ControlServerPort),
				},
			}, nil, types.NodeRoleClient)

			if err != nil {
				cmd.PrintErrf("Failed to create join request: %v\n", err)
				return
			}

			joinRequestBase64, err := joinRequest.ToBase64()

			if err != nil {
				cmd.PrintErrf("Failed to encode join request: %v\n", err)
				return
			}

			if !quietClientCreation {
				cmd.Printf("wireport:\n\nNew client created, use the following join request to connect to the network:\n\nwireport join %s\n", *joinRequestBase64)
			} else {
				cmd.Printf("%s\n", *joinRequestBase64)
			}
		}
	},
}

func init() {
	NewClientCmd.Flags().BoolVarP(&forceClientCreation, "force", "f", false, "Force create a client node without generating a join request")
	NewClientCmd.Flags().BoolVarP(&quietClientCreation, "quiet", "q", false, "Quiet mode, don't print any output except for the join request token")

	ClientCmd.AddCommand(NewClientCmd)
}
