package commands

import (
	"encoding/json"
	"os"

	"wireport/cmd/server/config"
	docker_utils "wireport/internal/docker-utils"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"

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

		if response.JoinConfigs.DockerSubnet == nil {
			logger.Fatal("Docker subnet is required to join the network")
			return
		}

		dockerSubnet, err := types.ParseIPNetMarshable(*response.JoinConfigs.DockerSubnet)

		if err != nil {
			logger.Fatal("Failed to parse docker subnet: %v", err)
			return
		}

		err = docker_utils.EnsureDockerNetworkExistsAndAttached(dockerSubnet)

		if err != nil {
			logger.Fatal("Failed to ensure docker network %s with subnet %s exists and is attached: %v", config.Config.DockerNetworkName, *response.JoinConfigs.DockerSubnet, err)
			return
		}

		if response.JoinConfigs.WireguardConfig == nil || response.JoinConfigs.CoreDNSConfig == nil {
			logger.Fatal("Wireguard and CoreDNS configs are required to join the network")
			return
		}

		logger.Info("Writing wireguard config to %s", config.Config.WireguardConfigPath)
		err = os.WriteFile(config.Config.WireguardConfigPath, []byte(*response.JoinConfigs.WireguardConfig), 0644)

		if err != nil {
			logger.Fatal("Failed to write wireguard config: %v", err)
			return
		}

		if response.JoinConfigs.CoreDNSConfig != nil {
			logger.Info("Writing coredns config to %s", config.Config.CoreDNSConfigPath)
			err = os.WriteFile(config.Config.CoreDNSConfigPath, []byte(*response.JoinConfigs.CoreDNSConfig), 0644)

			if err != nil {
				logger.Fatal("Failed to write coredns config: %v", err)
				return
			}
		}

		if response.JoinConfigs.ResolvConfig != nil {
			logger.Info("Writing resolv config to %s", config.Config.ResolvConfigPath)
			err = os.WriteFile(config.Config.ResolvConfigPath, []byte(*response.JoinConfigs.ResolvConfig), 0644)

			if err != nil {
				logger.Fatal("Failed to write resolv config: %v", err)
				return
			}
		}

		logger.Info("Successfully joined the network")
	},
}
