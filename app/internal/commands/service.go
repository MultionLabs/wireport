package commands

import (
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	commandstypes "wireport/internal/commands/types"
	"wireport/internal/joinrequests"
	"wireport/internal/nodes"
	"wireport/internal/nodes/types"
	"wireport/internal/publicservices"
	"wireport/internal/ssh"
)

type Service struct {
	LocalCommandsService     LocalCommandsService
	NodesRepository          *nodes.Repository
	PublicServicesRepository *publicservices.Repository
	JoinRequestsRepository   *joinrequests.Repository
}

// RoleHandler defines a function that can be executed for a specific role
type RoleHandler func() error

// RoleGroup defines a group of roles that share the same execution behavior
type RoleGroup struct {
	Roles   []types.NodeRole
	Handler RoleHandler
}

// handles the common pattern of command execution with role validation
func (s *Service) executeCommand(
	errorPrefix string,
	_ io.Writer,
	errOut io.Writer,
	roleGroups []RoleGroup,
) {
	currentNode, err := s.NodesRepository.GetCurrentNode()

	// Build the complete list of allowed roles from role groups
	var allowedRoles []types.NodeRole
	for _, group := range roleGroups {
		allowedRoles = append(allowedRoles, group.Roles...)
	}

	// Check if current node is allowed to execute this command
	if !s.isRoleAllowed(currentNode, allowedRoles) {
		if err != nil {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}
		s.printRoleError(errOut, allowedRoles)
		return
	}

	// Determine the role to use for execution
	var roleToExecute types.NodeRole
	if currentNode == nil {
		roleToExecute = types.NodeRoleEmpty
	} else {
		roleToExecute = currentNode.Role
	}

	// Find and execute the appropriate handler
	for _, group := range roleGroups {
		if slices.Contains(group.Roles, roleToExecute) {
			if err := group.Handler(); err != nil {
				fmt.Fprintf(errOut, "%s: %v\n", errorPrefix, err)
			}
			return
		}
	}

	// If no handler found, show error
	s.printRoleError(errOut, allowedRoles)
}

// locally executable commands
func (s *Service) createLocalHandler(localCall func()) RoleHandler {
	return func() error {
		localCall()
		return nil
	}
}

// remotely executable commands
func (s *Service) createAPIHandler(
	apiCall func(*APICommandsService) (commandstypes.ExecResponseDTO, error),
	stdOut io.Writer,
	errOut io.Writer,
	errorPrefix string,
) RoleHandler {
	return func() error {
		currentNode, err := s.NodesRepository.GetCurrentNode()

		if err != nil {
			return fmt.Errorf("failed to get current node: %v", err)
		}

		apiService := &APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		execResponseDTO, err := apiCall(apiService)
		if err != nil {
			return fmt.Errorf("%s: %v", errorPrefix, err)
		}

		if len(execResponseDTO.Stderr) > 0 {
			fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
		}

		fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
		return nil
	}
}

// if the current node role is allowed for the command
func (s *Service) isRoleAllowed(currentNode *types.Node, allowedRoles []types.NodeRole) bool {
	if currentNode == nil {
		return slices.Contains(allowedRoles, types.NodeRoleEmpty)
	}

	return slices.Contains(allowedRoles, currentNode.Role)
}

// standardized role error message
func (s *Service) printRoleError(errOut io.Writer, allowedRoles []types.NodeRole) {
	roleStrings := make([]string, len(allowedRoles))
	for i, role := range allowedRoles {
		roleStrings[i] = string(role)
	}
	fmt.Fprintf(errOut, "Error: unsupported command on this node\nOnly allowed on nodes: %s\n", strings.Join(roleStrings, ", "))
}

// gateway commands

func (s *Service) GatewayStart(gatewayPublicIP string, stdOut io.Writer, errOut io.Writer, gatewayStartConfigureOnly bool, router http.Handler) {
	s.executeCommand(
		"Failed to start gateway",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleEmpty},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.GatewayStart(gatewayPublicIP, stdOut, errOut, gatewayStartConfigureOnly, router)
				}),
			},
		},
	)
}

func (s *Service) GatewayStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.GatewayStatus(creds, stdOut)
}

func (s *Service) GatewayUp(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to bootstrap gateway node",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleEmpty},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.GatewayUp(creds, stdOut, errOut)
				}),
			},
		},
	)
}

func (s *Service) GatewayDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to tear down gateway node",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleClient},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.GatewayDown(creds, stdOut, errOut)
				}),
			},
		},
	)
}

func (s *Service) GatewayUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to upgrade gateway node",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.GatewayUpgrade(creds, stdOut, errOut)
				}),
			},
		},
	)
}

// server commands

func (s *Service) ServerNew(stdOut io.Writer, errOut io.Writer, forceServerCreation bool, quietServerCreation bool, dockerSubnet string) {
	s.executeCommand(
		"Failed to create server on the gateway",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet, stdOut, errOut)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet)
					},
					stdOut,
					errOut,
					"Failed to create server on the gateway",
				),
			},
		},
	)
}

func (s *Service) ServerRemove(stdOut io.Writer, errOut io.Writer, serverNodeID string) {
	s.executeCommand(
		"Failed to remove server",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServerRemove(serverNodeID)
					},
					stdOut,
					errOut,
					"Failed to remove server",
				),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerRemove(stdOut, errOut, serverNodeID)
				}),
			},
		},
	)
}

func (s *Service) ServerStart(stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to start server",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerStart(stdOut, errOut)
				}),
			},
		},
	)
}

func (s *Service) ServerStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.ServerStatus(creds, stdOut)
}

func (s *Service) ServerUp(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, dockerSubnet string) {
	s.executeCommand(
		"Failed to bootstrap server node",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerUp(creds, stdOut, errOut, dockerSubnet, s)
				}),
			},
		},
	)
}

func (s *Service) ServerDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to tear down server node",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerDown(creds, stdOut, errOut)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						// 1. Tear down server node
						s.LocalCommandsService.ServerDown(creds, stdOut, errOut)

						currentNode, err := s.NodesRepository.GetCurrentNode()

						if err != nil {
							return commandstypes.ExecResponseDTO{}, fmt.Errorf("failed to get current node: %v", err)
						}

						// 2. Remove server node from the gateway

						return api.ServerRemove(currentNode.ID)
					},
					stdOut,
					errOut,
					"Failed to tear down server node",
				),
			},
		},
	)
}

func (s *Service) ServerList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to list servers",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerList(requestFromNodeID, stdOut, errOut)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServerList()
					},
					stdOut,
					errOut,
					"Failed to list servers",
				),
			},
		},
	)
}

func (s *Service) ServerUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to upgrade server",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServerUpgrade(creds, stdOut, errOut)
				}),
			},
		},
	)
}

// client commands

func (s *Service) ClientNew(stdOut io.Writer, errOut io.Writer, joinRequestClientCreation bool, quietClientCreation bool, waitClientCreation bool) {
	s.executeCommand(
		"Failed to create client on the gateway",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleEmpty},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ClientNew(stdOut, errOut, joinRequestClientCreation, quietClientCreation, waitClientCreation)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ClientNew(joinRequestClientCreation, quietClientCreation, waitClientCreation)
					},
					stdOut,
					errOut,
					"Failed to create client on the gateway",
				),
			},
		},
	)
}

func (s *Service) ClientList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to list clients",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ClientList(requestFromNodeID, stdOut, errOut)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ClientList()
					},
					stdOut,
					errOut,
					"Failed to list clients",
				),
			},
		},
	)
}

// service commands

func (s *Service) ServicePublish(stdOut io.Writer, errOut io.Writer,
	localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		"Failed to publish service",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServicePublish(stdOut, errOut, localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServicePublish(localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
					},
					stdOut,
					errOut,
					"Failed to publish service",
				),
			},
		},
	)
}

func (s *Service) ServiceUnpublish(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		"Failed to unpublish service",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServiceUnpublish(stdOut, errOut, publicProtocol, publicHost, publicPort)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServiceUnpublish(publicProtocol, publicHost, publicPort)
					},
					stdOut,
					errOut,
					"Failed to unpublish service",
				),
			},
		},
	)
}

func (s *Service) ServiceList(stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		"Failed to list services",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServiceList(stdOut, errOut)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServiceList()
					},
					stdOut,
					errOut,
					"Failed to list services",
				),
			},
		},
	)
}

// service params commands

func (s *Service) ServiceParamNew(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	s.executeCommand(
		"Failed to add service param",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServiceParamNew(stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServiceParamNew(publicProtocol, publicHost, publicPort, paramType, paramValue)
					},
					stdOut,
					errOut,
					"Failed to add service param",
				),
			},
		},
	)
}

func (s *Service) ServiceParamRemove(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	s.executeCommand(
		"Failed to remove service param",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServiceParamRemove(stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServiceParamRemove(publicProtocol, publicHost, publicPort, paramType, paramValue)
					},
					stdOut,
					errOut,
					"Failed to remove service param",
				),
			},
		},
	)
}

func (s *Service) ServiceParamList(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		"Failed to list service params",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.ServiceParamList(stdOut, errOut, publicProtocol, publicHost, publicPort)
				}),
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: s.createAPIHandler(
					func(api *APICommandsService) (commandstypes.ExecResponseDTO, error) {
						return api.ServiceParamList(publicProtocol, publicHost, publicPort)
					},
					stdOut,
					errOut,
					"Failed to list service params",
				),
			},
		},
	)
}

// join command

func (s *Service) Join(stdOut io.Writer, errOut io.Writer, joinToken string) {
	s.executeCommand(
		"Failed to join network",
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleEmpty},
				Handler: s.createLocalHandler(func() {
					s.LocalCommandsService.Join(stdOut, errOut, joinToken)
				}),
			},
		},
	)
}
