package commands

import (
	"net"

	"wireport/cmd/server/config"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"

	"github.com/spf13/cobra"
)

var force bool = false
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
			logger.Fatal("Failed to count available Docker subnets: %v", err)
			return
		}

		totalJoinRequests := join_requests_repository.Count()

		if availableDockerSubnets <= 0 || totalJoinRequests >= availableDockerSubnets {
			logger.Fatal("No Docker subnets available. Please delete some server nodes (total: %d) or join-requests (total: %d) to free up some subnets.", totalDockerSubnets, totalJoinRequests)
			return
		}

		var dockerSubnetPtr *string

		if dockerSubnet != "" {
			// validate the subnet format
			parsedDockerSubnet, err := types.ParseIPNetMarshable(dockerSubnet, true)

			if err != nil {
				logger.Fatal("Failed to parse Docker subnet: %v", err)
				return
			}

			if !nodes_repository.IsDockerSubnetAvailable(parsedDockerSubnet) {
				logger.Fatal("Docker subnet %s is already in use", dockerSubnet)
				return
			}

			dockerSubnetPtr = &dockerSubnet

			logger.Info("Using custom Docker subnet: %s", dockerSubnet)
		}

		if force {
			logger.Info("Force flag detected, creating server node without generating a join request")

			_, err := nodes_repository.CreateServer(dockerSubnetPtr)

			if err != nil {
				logger.Fatal("Failed to create server node: %v", err)
				return
			}

			logger.Info("Server node created without join request")
			return
		}

		hostNode, err := nodes_repository.GetHostNode()

		if err != nil {
			logger.Fatal("Failed to get host node: %v", err)
			return
		}

		joinRequest, err := join_requests_repository.Create(types.UDPAddrMarshable{
			UDPAddr: net.UDPAddr{
				IP:   net.ParseIP(*hostNode.WGPublicIp),
				Port: int(config.Config.ControlServerPort),
			},
		}, dockerSubnetPtr)

		if err != nil {
			logger.Fatal("Failed to create join request: %v", err)
			return
		}

		joinRequestBase64, err := joinRequest.ToBase64()

		if err != nil {
			logger.Fatal("Failed to encode join request: %v", err)
			return
		}

		logger.Info("wireport:\n\nServer created, execute the command below on the server to join the network:\n\nwireport join %s", *joinRequestBase64)
	},
}

var StartServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the wireport server",
	Long:  `Start the wireport server. This command is only relevant for server nodes after they joined the network.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Starting wireport server")

		currentNode, err := nodes_repository.GetCurrentNode()

		if err != nil {
			logger.Fatal("Failed to get current node: %v", err)
			return
		}

		if currentNode == nil {
			logger.Fatal("No current node found")
			return
		}

		if currentNode.Role != types.NodeRoleServer {
			logger.Fatal("Current node is not a server node")
			return
		}

		publicServices := []*public_services.PublicService{}

		currentNode.SaveConfigs(publicServices, true)

		logger.Info("Server node configs saved to the disk successfully")
	},
}

func init() {
	NewServerCmd.Flags().BoolVarP(&force, "force", "f", false, "Force the creation of a new server, bypassing the join request generation")
	NewServerCmd.Flags().StringVar(&dockerSubnet, "docker-subnet", "", "Specify a custom Docker subnet for the server (e.g. 172.20.0.0/16)")
	ServerCmd.AddCommand(NewServerCmd)
	ServerCmd.AddCommand(StartServerCmd)
}
