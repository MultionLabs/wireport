package commands

import (
	"encoding/json"

	"wireport/cmd/server/config"
	docker_utils "wireport/internal/docker-utils"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"

	"github.com/spf13/cobra"
)

var JoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join the wireport network",
	Long:  `Join the wireport network using a join-request token, provided by the 'wireport server create' command`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			logger.Fatal("Provide a join token")
			return
		}

		joinToken := args[0]

		if joinToken == "" {
			logger.Fatal("Provide a join token")
			return
		}

		logger.Info("Joining wireport network with token: %s", joinToken)

		response, err := join_requests_service.Join(joinToken)

		if err != nil {
			logger.Fatal("Failed to join network: %v", err)
			return
		}

		responseBytes, _ := json.Marshal(response)
		responseJSON := string(responseBytes)

		logger.Info("Join response: %s", responseJSON)

		currentNode := response.NodeConfig

		if currentNode != nil {
			logger.Info("Saving node config to database")

			dockerSubnet, err := types.ParseIPNetMarshable(currentNode.DockerSubnet.String(), true)

			if err != nil {
				logger.Fatal("Failed to parse docker subnet: %v", err)
				return
			}

			err = docker_utils.EnsureDockerNetworkExistsAndAttached(dockerSubnet)

			if err != nil {
				logger.Fatal("Failed to ensure docker network %s with subnet %s exists and is attached: %v", config.Config.DockerNetworkName, dockerSubnet.String(), err)
				return
			}

			currentNode.IsCurrentNode = true

			err = nodes_repository.SaveNode(currentNode)

			if err != nil {
				logger.Fatal("Failed to save node config: %v", err)
				return
			}

			publicServices := []*public_services.PublicService{}

			err = currentNode.SaveConfigs(publicServices, true)

			if err != nil {
				logger.Fatal("Failed to save node configs: %v", err)
				return
			}

			logger.Info("Successfully saved node config to the database")
		}

		logger.Info("Successfully joined the network")
	},
}
