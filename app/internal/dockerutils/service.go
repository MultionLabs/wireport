package dockerutils

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"wireport/cmd/server/config"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
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

	cli, err := client.New(client.FromEnv)

	if err != nil {
		return nil, err
	}

	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})

	if err != nil {
		return nil, err
	}

	containerName := strings.TrimPrefix(inspect.Container.Name, "/")

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
	cli, err := client.New(client.FromEnv)

	if err != nil {
		return err
	}

	defer cli.Close()

	subnetPrefix, err := netip.ParsePrefix(dockerSubnet.String())

	if err != nil {
		return err
	}

	// check if network exists

	_, err = cli.NetworkInspect(context.Background(), config.Config.DockerNetworkName, client.NetworkInspectOptions{})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Info("Network does not exist, creating network %s with subnet %s", config.Config.DockerNetworkName, dockerSubnet.String())

			_, err = cli.NetworkCreate(context.Background(), config.Config.DockerNetworkName, client.NetworkCreateOptions{
				Driver: config.Config.DockerNetworkDriver,
				IPAM: &network.IPAM{
					Config: []network.IPAMConfig{
						{
							Subnet: subnetPrefix,
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

	inspect, err := cli.ContainerInspect(context.Background(), *containerName, client.ContainerInspectOptions{})

	if err != nil {
		logger.Error("Failed to inspect container")
		return err
	}

	networks := map[string]*network.EndpointSettings{}

	if inspect.Container.NetworkSettings != nil && inspect.Container.NetworkSettings.Networks != nil {
		networks = inspect.Container.NetworkSettings.Networks
	}

	for networkName := range networks {
		if networkName == config.Config.DockerNetworkName {
			logger.Info("Container %s is already connected to network %s", *containerName, config.Config.DockerNetworkName)
			return nil
		}
	}

	logger.Info("Connecting container %s to network %s", *containerName, config.Config.DockerNetworkName)

	_, err = cli.NetworkConnect(context.Background(), config.Config.DockerNetworkName, client.NetworkConnectOptions{
		Container:      *containerName,
		EndpointConfig: &network.EndpointSettings{},
	})

	if err != nil {
		logger.Error("Failed to connect container to network")
		return err
	}

	logger.Info("Container connected to network")

	return nil
}

func EnsureDockerNetworkIsAttachedToAllContainers() error {
	cli, err := client.New(client.FromEnv)

	if err != nil {
		return err
	}

	defer cli.Close()

	listResult, err := cli.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})

	if err != nil {
		return err
	}

	for _, ctr := range listResult.Items {
		isGateway := slices.Contains(ctr.Names, fmt.Sprintf("/%s", config.Config.WireportGatewayContainerName))

		if isGateway {
			// Skip wireport-gateway container (should not be connected to the network)
			continue
		}

		// Skip containers that are already connected to the target network to avoid conflicts
		if ctr.NetworkSettings != nil {
			if _, ok := ctr.NetworkSettings.Networks[config.Config.DockerNetworkName]; ok {
				continue
			}
		}

		_, err = cli.NetworkConnect(context.Background(), config.Config.DockerNetworkName, client.NetworkConnectOptions{
			Container:      ctr.ID,
			EndpointConfig: &network.EndpointSettings{},
		})

		if err != nil {
			// If Docker reports that connecting the network would create an IP / route conflict we simply skip this container and continue with the others.
			if strings.Contains(err.Error(), "conflicts with existing route") ||
				strings.Contains(err.Error(), "cannot program address") ||
				cerrdefs.IsConflict(err) || cerrdefs.IsPermissionDenied(err) {
				continue
			}

			return err
		}

		logger.Info("Container %s is now connected to network %s", ctr.Names[0], config.Config.DockerNetworkName)
	}

	return nil
}

func DetachDockerNetworkFromAllContainers() error {
	cli, err := client.New(client.FromEnv)

	if err != nil {
		return err
	}

	defer cli.Close()

	listResult, err := cli.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})

	if err != nil {
		return err
	}

	for _, ctr := range listResult.Items {
		// If the container is not connected to the target network we can safely skip it.
		if ctr.NetworkSettings != nil {
			if _, ok := ctr.NetworkSettings.Networks[config.Config.DockerNetworkName]; !ok {
				continue
			}
		}

		_, err = cli.NetworkDisconnect(context.Background(), config.Config.DockerNetworkName, client.NetworkDisconnectOptions{
			Container: ctr.ID,
			Force:     true,
		})

		if err != nil {
			if cerrdefs.IsNotFound(err) || cerrdefs.IsPermissionDenied(err) || cerrdefs.IsConflict(err) {
				continue
			}

			return err
		}

		logger.Info("Container %s is now disconnected from network %s", ctr.Names[0], config.Config.DockerNetworkName)
	}

	return nil
}

func RemoveDockerNetwork() error {
	cli, err := client.New(client.FromEnv)

	if err != nil {
		return err
	}

	defer cli.Close()

	// check if network exists

	_, err = cli.NetworkInspect(context.Background(), config.Config.DockerNetworkName, client.NetworkInspectOptions{})

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}

		return err
	}

	// remove network

	_, err = cli.NetworkRemove(context.Background(), config.Config.DockerNetworkName, client.NetworkRemoveOptions{})

	if err != nil {
		return err
	}

	return nil
}

func ListAllContainerLabels() (map[string]map[string]string, error) {
	cli, err := client.New(client.FromEnv)

	if err != nil {
		return nil, err
	}

	defer cli.Close()

	listResult, err := cli.ContainerList(context.Background(), client.ContainerListOptions{
		All: true,
	})

	if err != nil {
		return nil, err
	}

	containerLabels := make(map[string]map[string]string)

	for _, ctr := range listResult.Items {
		// container name without leading slash
		containerName := strings.TrimPrefix(ctr.Names[0], "/")

		inspect, err := cli.ContainerInspect(context.Background(), ctr.ID, client.ContainerInspectOptions{})

		if err != nil {
			logger.Error("Failed to inspect container %s: %v", containerName, err)
			continue
		}

		if inspect.Container.Config != nil {
			containerLabels[containerName] = inspect.Container.Config.Labels
		} else {
			containerLabels[containerName] = nil
		}
	}

	return containerLabels, nil
}
