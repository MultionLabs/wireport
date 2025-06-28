package dockerutils

import (
	"context"
	"os"
	"strings"
	"wireport/cmd/server/config"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func getRunningContainerID() (*string, error) {
	hostname, err := os.Hostname()

	if err != nil {
		return nil, err
	}

	return &hostname, nil
}

func getContainerName(containerID string) (*string, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return nil, err
	}

	cli.NegotiateAPIVersion(ctx)

	containerJSON, err := cli.ContainerInspect(ctx, containerID)

	if err != nil {
		return nil, err
	}

	containerName := strings.TrimPrefix(containerJSON.Name, "/")

	return &containerName, nil
}

func getRunningContainerName() (*string, error) {
	containerID, err := getRunningContainerID()

	if err != nil {
		return nil, err
	}

	containerName, err := getContainerName(*containerID)

	if err != nil {
		return nil, err
	}

	return containerName, nil
}

// ------------------------------------------------------------------------------------------------

func EnsureDockerNetworkExistsAndAttached(dockerSubnet *types.IPNetMarshable) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	if err != nil {
		return err
	}

	// check if network exists

	_, err = cli.NetworkInspect(context.Background(), config.Config.DockerNetworkName, network.InspectOptions{})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("Network does not exist, creating network %s with subnet %s", config.Config.DockerNetworkName, dockerSubnet.String())

			_, err = cli.NetworkCreate(context.Background(), config.Config.DockerNetworkName, network.CreateOptions{
				Driver: config.Config.DockerNetworkDriver,
				IPAM: &network.IPAM{
					Config: []network.IPAMConfig{
						{
							Subnet: dockerSubnet.String(),
						},
					},
				},
			})

			if err != nil {
				logger.Error("Failed to create network")
				return err
			}

			logger.Info("Network created")
		} else {
			return err
		}
	} else {
		logger.Info("Network already exists")
	}

	containerName, err := getRunningContainerName()

	if err != nil {
		logger.Error("Failed to get running container name")
		return err
	}

	containerJSON, err := cli.ContainerInspect(context.Background(), *containerName)

	if err != nil {
		logger.Error("Failed to inspect container")
		return err
	}

	for networkName := range containerJSON.NetworkSettings.Networks {
		if networkName == config.Config.DockerNetworkName {
			logger.Info("Container %s is already connected to network %s", *containerName, config.Config.DockerNetworkName)
			return nil
		}
	}

	logger.Info("Connecting container %s to network %s", *containerName, config.Config.DockerNetworkName)

	err = cli.NetworkConnect(context.Background(), config.Config.DockerNetworkName, *containerName, &network.EndpointSettings{})

	if err != nil {
		logger.Error("Failed to connect container to network")
		return err
	}

	logger.Info("Container connected to network")

	defer cli.Close()

	return nil
}
