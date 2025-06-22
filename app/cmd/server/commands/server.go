package commands

import (
	"net"

	"wireport/cmd/server/config"
	"wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"

	"github.com/spf13/cobra"
)

var forceServerCreation bool = false
var quietServerCreation bool = false
var dockerSubnet string = ""

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "wireport server commands",
	Long:  `Manage connected wireport server nodes and create join-requests for connecting new servers to the wireport network`,
}

var NewServerCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a server to wireport network",
	Long:  `Create a new join-request for connecting a server to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command)`,
	Run: func(cmd *cobra.Command, args []string) {
		totalDockerSubnets, availableDockerSubnets, err := nodes_repository.TotalAndAvailableDockerSubnets()

		if err != nil {
			cmd.PrintErrf("Failed to count available Docker subnets: %v\n", err)
			return
		}

		totalServerRoleJoinRequests := join_requests_repository.CountServerJoinRequests()

		if availableDockerSubnets <= 0 || totalServerRoleJoinRequests >= availableDockerSubnets {
			cmd.PrintErrf("No Docker subnets available. Please delete some server nodes (total used: %d) or server join-requests (total used: %d) to free up some subnets.\n", totalDockerSubnets, totalServerRoleJoinRequests)
			return
		}

		totalWireguardClients, availableWireguardClients, err := nodes_repository.TotalAvailableWireguardClients()

		if err != nil {
			cmd.PrintErrf("Failed to count available Wireguard clients: %v\n", err)
			return
		}

		totalJoinRequests := join_requests_repository.CountAll()

		if availableWireguardClients <= 0 || totalJoinRequests >= availableWireguardClients {
			cmd.PrintErrf("No Wireguard clients available. Please delete some client/server nodes (total used: %d) or client/server join-requests (total used: %d) to free up some clients.\n", totalWireguardClients, totalJoinRequests)
			return
		}

		var dockerSubnetPtr *string

		if dockerSubnet != "" {
			// validate the subnet format
			parsedDockerSubnet, err := types.ParseIPNetMarshable(dockerSubnet, true)

			if err != nil {
				cmd.PrintErrf("Failed to parse Docker subnet: %v\n", err)
				return
			}

			if !nodes_repository.IsDockerSubnetAvailable(parsedDockerSubnet) {
				cmd.PrintErrf("Docker subnet %s is already in use\n", dockerSubnet)
				return
			}

			dockerSubnetPtr = &dockerSubnet

			if !quietServerCreation {
				cmd.Printf("Using custom Docker subnet: %s\n", dockerSubnet)
			}
		}

		if forceServerCreation {
			if !quietServerCreation {
				cmd.Printf("Force flag detected, creating server node without generating a join request\n")
			}

			_, err := nodes_repository.CreateServer(dockerSubnetPtr)

			if err != nil {
				cmd.PrintErrf("Failed to create server node: %v\n", err)
				return
			}

			if !quietServerCreation {
				cmd.Printf("Server node created without join request\n")
			}

			return
		}

		hostNode, err := nodes_repository.GetHostNode()

		if err != nil {
			cmd.PrintErrf("Failed to get host node: %v\n", err)
			return
		}

		joinRequest, err := join_requests_repository.Create(types.UDPAddrMarshable{
			UDPAddr: net.UDPAddr{
				IP:   net.ParseIP(*hostNode.WGPublicIp),
				Port: int(config.Config.ControlServerPort),
			},
		}, dockerSubnetPtr, types.NodeRoleServer)

		if err != nil {
			cmd.PrintErrf("Failed to create join request: %v\n", err)
			return
		}

		joinRequestBase64, err := joinRequest.ToBase64()

		if err != nil {
			cmd.PrintErrf("Failed to encode join request: %v\n", err)
			return
		}

		if !quietServerCreation {
			cmd.Printf("wireport:\n\nServer created, execute the command below on the server to join the network:\n\nwireport join %s\n", *joinRequestBase64)
		} else {
			cmd.Printf("%s\n", *joinRequestBase64)
		}
	},
}

var StartServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the wireport server",
	Long:  `Start the wireport server. This command is only relevant for server nodes after they joined the network.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("Starting wireport server\n")

		currentNode, err := nodes_repository.GetCurrentNode()

		if err != nil {
			cmd.PrintErrf("Failed to get current node: %v\n", err)
			return
		}

		if currentNode == nil {
			cmd.PrintErrf("No current node found\n")
			return
		}

		if currentNode.Role != types.NodeRoleServer {
			cmd.PrintErrf("Current node is not a server node\n")
			return
		}

		publicServices := []*public_services.PublicService{}

		currentNode.SaveConfigs(publicServices, true)

		cmd.Printf("Server node configs saved to the disk successfully\n")
	},
}

func init() {
	NewServerCmd.Flags().BoolVarP(&forceServerCreation, "force", "f", false, "Force the creation of a new server, bypassing the join request generation")
	NewServerCmd.Flags().StringVar(&dockerSubnet, "docker-subnet", "", "Specify a custom Docker subnet for the server (e.g. 172.20.0.0/16)")
	NewServerCmd.Flags().BoolVarP(&quietServerCreation, "quiet", "q", false, "Quiet mode, don't print any output except for the join request token")

	ServerCmd.AddCommand(NewServerCmd)
	ServerCmd.AddCommand(StartServerCmd)
}
