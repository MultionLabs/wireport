package commands

import (
	"net"

	"wireport/cmd/server/config"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"

	"github.com/spf13/cobra"
)

var force bool = false

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
		if force {
			logger.Info("Force flag detected, creating server node without generating a join request")

			_, err := nodes_repository.CreateServer()

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
		})

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

func init() {
	NewServerCmd.Flags().BoolVarP(&force, "force", "f", false, "Force the creation of a new server, bypassing the join request generation")
	ServerCmd.AddCommand(NewServerCmd)
}
