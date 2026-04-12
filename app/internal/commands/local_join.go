package commands

import (
	"fmt"
	"io"
	"wireport/cmd/server/config"
	"wireport/internal/dockerutils"
	"wireport/internal/joinrequests"
	joinrequeststypes "wireport/internal/joinrequests/types"
	"wireport/internal/nodes/types"
	"wireport/internal/publicservices"
)

func (s *LocalCommandsService) Join(stdOut io.Writer, errOut io.Writer, joinToken string) bool {
	joinRequest := &joinrequeststypes.JoinRequest{}

	err := joinRequest.FromBase64(joinToken)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to parse join request: %v\n", err)
		return false
	}

	joinRequestsService := joinrequests.NewAPIService(&joinRequest.ClientCertBundle)

	response, err := joinRequestsService.Join(joinToken, fmt.Sprintf("%s:%d", joinRequest.GatewayHost, joinRequest.GatewayPort))

	if err != nil {
		fmt.Fprintf(errOut, "Failed to join network: %v\n", err)
		return false
	}

	currentNode := response.NodeConfig

	if currentNode == nil {
		fmt.Fprintf(errOut, "Failed to get node config from join response\n")
		return false
	}

	currentNode.IsCurrentNode = true

	err = s.NodesRepository.SaveNode(currentNode)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save node config: %v\n", err)
		return false
	}

	switch currentNode.Role {
	case types.NodeRoleServer:
		fmt.Fprintf(stdOut, "Setting up server node (configs and network)\n")

		if currentNode.DockerSubnet == nil {
			fmt.Fprintf(errOut, "Failed to get docker subnet from node config\n")
			return false
		}

		dockerSubnet, err := types.ParseIPNetMarshable(currentNode.DockerSubnet.String(), true)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to parse docker subnet: %v\n", err)
			return false
		}

		err = dockerutils.EnsureDockerNetworkExistsAndAttached(dockerSubnet)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to ensure docker network %s with subnet %s exists and is attached: %v\n", config.Config.DockerNetworkName, dockerSubnet.String(), err)
			return false
		}

		publicServices := []*publicservices.PublicService{}

		err = currentNode.SaveConfigs(publicServices, true)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to save node configs: %v\n", err)
			return false
		}

		fmt.Fprintf(stdOut, "Successfully saved node config to the database\n")
	case types.NodeRoleClient:
		wireguardConfig, err := currentNode.GetFormattedWireguardConfig()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to get wireguard config: %v\n", err)
			return false
		}

		fmt.Fprintf(stdOut, "\n%s\n", *wireguardConfig)
		fmt.Fprintf(errOut, "\n⤵ wireport WireGuard config has been dumped\n\n")
	default:
		fmt.Fprintf(errOut, "Invalid node role: %s\n", currentNode.Role)
		return false
	}

	return true
}
