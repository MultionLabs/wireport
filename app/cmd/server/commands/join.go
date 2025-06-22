package commands

import (
	"encoding/json"

	"wireport/cmd/server/config"
	docker_utils "wireport/internal/docker-utils"
	"wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"

	"github.com/spf13/cobra"
)

var JoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join the wireport network",
	Long:  `Join the wireport network using a join-request token, provided by the 'wireport server new' command`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.PrintErrf("Provide a join token\n")
			return
		}

		joinToken := args[0]

		if joinToken == "" {
			cmd.PrintErrf("Provide a join token\n")
			return
		}

		cmd.Printf("Joining wireport network with token: %s\n", joinToken)

		response, err := join_requests_service.Join(joinToken)

		if err != nil {
			cmd.PrintErrf("Failed to join network: %v\n", err)
			return
		}

		responseBytes, _ := json.Marshal(response)
		responseJSON := string(responseBytes)

		cmd.Printf("Join response: %s\n", responseJSON)

		currentNode := response.NodeConfig

		if currentNode != nil {
			cmd.Printf("Saving node config to database\n")

			dockerSubnet, err := types.ParseIPNetMarshable(currentNode.DockerSubnet.String(), true)

			if err != nil {
				cmd.PrintErrf("Failed to parse docker subnet: %v\n", err)
				return
			}

			err = docker_utils.EnsureDockerNetworkExistsAndAttached(dockerSubnet)

			if err != nil {
				cmd.PrintErrf("Failed to ensure docker network %s with subnet %s exists and is attached: %v\n", config.Config.DockerNetworkName, dockerSubnet.String(), err)
				return
			}

			currentNode.IsCurrentNode = true

			err = nodes_repository.SaveNode(currentNode)

			if err != nil {
				cmd.PrintErrf("Failed to save node config: %v\n", err)
				return
			}

			publicServices := []*public_services.PublicService{}

			err = currentNode.SaveConfigs(publicServices, true)

			if err != nil {
				cmd.PrintErrf("Failed to save node configs: %v\n", err)
				return
			}

			cmd.Printf("Successfully saved node config to the database\n")
		}

		cmd.Printf("Successfully joined the network\n")
	},
}
