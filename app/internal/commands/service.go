package commands

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	commandstypes "wireport/internal/commands/types"
	"wireport/internal/joinrequests"
	"wireport/internal/nodes"
	"wireport/internal/nodes/types"
	"wireport/internal/publicservices"
	"wireport/internal/ssh"
)

type Service struct {
	LocalCommandsService LocalCommandsService
}

// gateway

func (s *Service) GatewayStart(gatewayPublicIP string, nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository,
	stdOut io.Writer, errOut io.Writer, gatewayStartConfigureOnly bool, router http.Handler) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode == nil:
	case currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.GatewayStart(gatewayPublicIP, nodesRepository, publicServicesRepository, stdOut, errOut, gatewayStartConfigureOnly, router)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleGateway), string(types.NodeRoleNonInitialized)}, ", "))
		return
	}
}

func (s *Service) GatewayStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.GatewayStatus(creds, stdOut)
}

func (s *Service) GatewayUp(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode == nil:
		s.LocalCommandsService.GatewayUp(creds, nodesRepository, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleNonInitialized)}, ", "))
		return
	}
}

func (s *Service) GatewayDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && (currentNode.Role == types.NodeRoleGateway || currentNode.Role == types.NodeRoleClient):
		s.LocalCommandsService.GatewayDown(creds, stdOut, errOut, nodesRepository)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleGateway), string(types.NodeRoleClient)}, ", "))
		return
	}
}

func (s *Service) GatewayUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		s.LocalCommandsService.GatewayUpgrade(creds, stdOut, errOut, nodesRepository)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient)}, ", "))
		return
	}
}

// server

func (s *Service) ServerNew(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, stdOut io.Writer, errOut io.Writer, forceServerCreation bool, quietServerCreation bool, dockerSubnet string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to create server on the gateway: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet, nodesRepository, joinRequestsRepository, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServerStart(nodesRepository *nodes.Repository, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleServer:
		s.LocalCommandsService.ServerStart(nodesRepository, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleServer)}, ", "))
		return
	}
}

func (s *Service) ServerStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.ServerStatus(creds, stdOut)
}

func (s *Service) ServerUp(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, dockerSubnet string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		s.LocalCommandsService.ServerUp(nodesRepository, joinRequestsRepository, creds, stdOut, errOut, dockerSubnet)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient)}, ", "))
		return
	}
}

func (s *Service) ServerDown(nodesRepository *nodes.Repository, creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && (currentNode.Role == types.NodeRoleClient || currentNode.Role == types.NodeRoleServer):
		s.LocalCommandsService.ServerDown(nodesRepository, creds, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleServer)}, ", "))
		return
	}
}

func (s *Service) ServerList(nodesRepository *nodes.Repository, requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServerList()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list servers: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServerList(nodesRepository, requestFromNodeID, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServerUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		s.LocalCommandsService.ServerUpgrade(creds, stdOut, errOut, nodesRepository)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient)}, ", "))
		return
	}
}

// client

func (s *Service) ClientNew(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, publicServicesRepository *publicservices.Repository,
	stdOut io.Writer, errOut io.Writer, joinRequestClientCreation bool, quietClientCreation bool, waitClientCreation bool) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode == nil || currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ClientNew(nodesRepository, joinRequestsRepository, publicServicesRepository, stdOut, errOut, joinRequestClientCreation, quietClientCreation, waitClientCreation)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ClientNew(joinRequestClientCreation, quietClientCreation, waitClientCreation)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to create client on the gateway: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleGateway), string(types.NodeRoleClient), string(types.NodeRoleNonInitialized)}, ", "))
		return
	}
}

func (s *Service) ClientList(nodesRepository *nodes.Repository, requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ClientList()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list clients: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ClientList(nodesRepository, requestFromNodeID, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

// service

func (s *Service) ServicePublish(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer,
	localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServicePublish(localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to publish service: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServicePublish(nodesRepository, publicServicesRepository, stdOut, errOut, localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServiceUnpublish(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServiceUnpublish(publicProtocol, publicHost, publicPort)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to unpublish service: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServiceUnpublish(nodesRepository, publicServicesRepository, stdOut, errOut, publicProtocol, publicHost, publicPort)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServiceList(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServiceList()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServiceList(nodesRepository, publicServicesRepository, stdOut, errOut)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

// service params

func (s *Service) ServiceParamNew(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServiceParamNew(publicProtocol, publicHost, publicPort, paramType, paramValue)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to add service param: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServiceParamNew(nodesRepository, publicServicesRepository, stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServiceParamRemove(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServiceParamRemove(publicProtocol, publicHost, publicPort, paramType, paramValue)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to remove service param: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServiceParamRemove(nodesRepository, publicServicesRepository, stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

func (s *Service) ServiceParamList(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode != nil && currentNode.Role == types.NodeRoleClient:
		var execResponseDTO commandstypes.ExecResponseDTO

		apiService := APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err = apiService.ServiceParamList(publicProtocol, publicHost, publicPort)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list service params: %v\n", err)
			return
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return
	case currentNode != nil && currentNode.Role == types.NodeRoleGateway:
		s.LocalCommandsService.ServiceParamList(nodesRepository, publicServicesRepository, stdOut, errOut, publicProtocol, publicHost, publicPort)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleClient), string(types.NodeRoleGateway)}, ", "))
		return
	}
}

// join

func (s *Service) Join(nodesRepository *nodes.Repository, stdOut io.Writer, errOut io.Writer, joinToken string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	switch {
	case currentNode == nil:
		s.LocalCommandsService.Join(nodesRepository, stdOut, errOut, joinToken)
		return
	default:
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}

		fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join([]string{string(types.NodeRoleNonInitialized)}, ", "))
	}
}
