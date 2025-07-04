package commands

import (
	"errors"
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

	"gorm.io/gorm"
)

type Service struct {
	LocalCommandsService     LocalCommandsService
	NodesRepository          *nodes.Repository
	PublicServicesRepository *publicservices.Repository
	JoinRequestsRepository   *joinrequests.Repository
}

// RoleHandler defines a function that can be executed for a specific role
type RoleHandler func(currentNode *types.Node, api *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error)

// RoleGroup defines a group of roles that share the same execution behavior
type RoleGroup struct {
	Roles   []types.NodeRole
	Handler RoleHandler
}

// handles the common pattern of command execution with role validation
func (s *Service) executeCommand(
	stdOut io.Writer,
	errOut io.Writer,
	roleGroups []RoleGroup,
) {
	currentNode, err := s.NodesRepository.GetCurrentNode()

	if err != nil {
		// if node is not found -- it's ok, we consider it as node role = empty
		// otherwise, return the error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Fprintf(errOut, "Error: %v\n", err)
			return
		}
	}

	// Build the complete list of allowed roles from role groups
	var allowedRoles []types.NodeRole
	for _, group := range roleGroups {
		allowedRoles = append(allowedRoles, group.Roles...)
	}

	// Determine the role to use for execution
	var roleToExecute types.NodeRole
	if currentNode == nil {
		roleToExecute = types.NodeRoleEmpty
	} else {
		roleToExecute = currentNode.Role
	}

	// Check if current node is allowed to execute this command
	if !s.isRoleAllowed(currentNode, allowedRoles) {
		s.printRoleError(roleToExecute, errOut, allowedRoles)
		return
	}

	var apiService *APICommandsService

	if currentNode != nil {
		apiService = &APICommandsService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}
	}

	// Find and execute the appropriate handler
	for _, group := range roleGroups {
		if slices.Contains(group.Roles, roleToExecute) {
			execResponseDTO, err := group.Handler(currentNode, apiService, &s.LocalCommandsService)

			if err != nil {
				fmt.Fprintf(errOut, "%v\n", err)
				return
			}

			if execResponseDTO != nil {
				if len(execResponseDTO.Stderr) > 0 {
					fmt.Fprintf(errOut, "%s\n", execResponseDTO.Stderr)
				}

				fmt.Fprintf(stdOut, "%s\n", execResponseDTO.Stdout)
			}
			return
		}
	}

	// If no handler found, show error
	s.printRoleError(roleToExecute, errOut, allowedRoles)
}

// if the current node role is allowed for the command
func (s *Service) isRoleAllowed(currentNode *types.Node, allowedRoles []types.NodeRole) bool {
	if currentNode == nil {
		return slices.Contains(allowedRoles, types.NodeRoleEmpty)
	}

	return slices.Contains(allowedRoles, currentNode.Role)
}

// standardized role error message
func (s *Service) printRoleError(currentNodeRole types.NodeRole, errOut io.Writer, allowedRoles []types.NodeRole) {
	roleStrings := make([]string, len(allowedRoles))
	for i, role := range allowedRoles {
		roleStrings[i] = string(role)
	}
	fmt.Fprintf(errOut, "❌  Command is only allowed for roles: %s\n", strings.Join(roleStrings, ", "))
	fmt.Fprintf(errOut, "👤 Current role: %s\n\n", currentNodeRole)
	fmt.Fprintf(errOut, "To execute this command on behalf of:\n")
	fmt.Fprintf(errOut, "- client: 'wireport gateway up' OR 'wireport join <client-join-token>'\n")
	fmt.Fprintf(errOut, "- server: 'wireport gateway up' + 'wireport server up' + SSH to server container\n")
	fmt.Fprintf(errOut, "- gateway: 'wireport gateway up' + SSH to gateway container\n")
	fmt.Fprintf(errOut, "- empty: 'wireport gateway down' (WARNING: destroys all data)\n")
}

// gateway commands

func (s *Service) GatewayStart(gatewayPublicIP string, stdOut io.Writer, errOut io.Writer, gatewayStartConfigureOnly bool, router http.Handler) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleEmpty},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.GatewayStart(gatewayPublicIP, stdOut, errOut, gatewayStartConfigureOnly, router)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) GatewayStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.GatewayStatus(creds, stdOut)
}

func (s *Service) GatewayUp(creds *ssh.Credentials, imageTag string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleEmpty},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.GatewayUp(creds, imageTag, stdOut, errOut)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) GatewayDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleClient},
				Handler: func(currentNode *types.Node, api *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					if currentNode == nil {
						return nil, fmt.Errorf("current node is required to tear down gateway node")
					}

					if currentNode.Role == types.NodeRoleClient {
						serverListResult, err := api.ServerList()

						if err != nil {
							return nil, err
						}

						if serverListResult.ServerNodesCount > 0 {
							return nil, fmt.Errorf("❌ Cannot tear down gateway node with active server nodes (%d), please tear down all server nodes first", serverListResult.ServerNodesCount)
						}
					}

					local.GatewayDown(creds, stdOut, errOut)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) GatewayUpgrade(creds *ssh.Credentials, imageTag string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.GatewayUpgrade(creds, imageTag, stdOut, errOut)
					return nil, nil
				},
			},
		},
	)
}

// server commands

func (s *Service) ServerNew(stdOut io.Writer, errOut io.Writer, forceServerCreation bool, quietServerCreation bool, dockerSubnet string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet, stdOut, errOut)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServerNew(forceServerCreation, quietServerCreation, dockerSubnet)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServerRemove(stdOut io.Writer, errOut io.Writer, serverNodeID string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServerRemove(serverNodeID)
					return &execResponseDTO, err
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerRemove(stdOut, errOut, serverNodeID)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) ServerStart(stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerStart(stdOut, errOut)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) ServerStatus(creds *ssh.Credentials, stdOut io.Writer) {
	s.LocalCommandsService.ServerStatus(creds, stdOut)
}

func (s *Service) ServerUp(creds *ssh.Credentials, imageTag string, stdOut io.Writer, errOut io.Writer, dockerSubnet string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerUp(creds, imageTag, stdOut, errOut, dockerSubnet, s)
					return nil, nil
				},
			},
		},
	)
}

func (s *Service) ServerDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerDown(creds, stdOut, errOut)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleServer},
				Handler: func(currentNode *types.Node, api *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					if currentNode == nil {
						return nil, fmt.Errorf("current node is required to tear down server node")
					}

					// 1. Tear down server node
					local.ServerDown(creds, stdOut, errOut)

					// 2. Remove server node from the gateway
					execResponseDTO, err := api.ServerRemove(currentNode.ID)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServerList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerList(requestFromNodeID, stdOut, errOut)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					serverListResult, err := api.ServerList()
					return &serverListResult.ExecResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServerUpgrade(creds *ssh.Credentials, imageTag string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServerUpgrade(creds, imageTag, stdOut, errOut)
					return nil, nil
				},
			},
		},
	)
}

// client commands

func (s *Service) ClientNew(stdOut io.Writer, errOut io.Writer, joinRequestClientCreation bool, quietClientCreation bool, waitClientCreation bool) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway, types.NodeRoleEmpty},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ClientNew(stdOut, errOut, joinRequestClientCreation, quietClientCreation, waitClientCreation)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ClientNew(joinRequestClientCreation, quietClientCreation, waitClientCreation)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ClientList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ClientList(requestFromNodeID, stdOut, errOut)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ClientList()
					return &execResponseDTO, err
				},
			},
		},
	)
}

// service commands

func (s *Service) ServicePublish(stdOut io.Writer, errOut io.Writer,
	localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServicePublish(stdOut, errOut, localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServicePublish(localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServiceUnpublish(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServiceUnpublish(stdOut, errOut, publicProtocol, publicHost, publicPort)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServiceUnpublish(publicProtocol, publicHost, publicPort)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServiceList(stdOut io.Writer, errOut io.Writer) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServiceList(stdOut, errOut)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServiceList()
					return &execResponseDTO, err
				},
			},
		},
	)
}

// service params commands

func (s *Service) ServiceParamNew(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServiceParamNew(stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServiceParamNew(publicProtocol, publicHost, publicPort, paramType, paramValue)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServiceParamRemove(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServiceParamRemove(stdOut, errOut, publicProtocol, publicHost, publicPort, paramType, paramValue)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServiceParamRemove(publicProtocol, publicHost, publicPort, paramType, paramValue)
					return &execResponseDTO, err
				},
			},
		},
	)
}

func (s *Service) ServiceParamList(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleGateway},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.ServiceParamList(stdOut, errOut, publicProtocol, publicHost, publicPort)
					return nil, nil
				},
			},
			{
				Roles: []types.NodeRole{types.NodeRoleClient},
				Handler: func(_ *types.Node, api *APICommandsService, _ *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					execResponseDTO, err := api.ServiceParamList(publicProtocol, publicHost, publicPort)
					return &execResponseDTO, err
				},
			},
		},
	)
}

// join command

func (s *Service) Join(stdOut io.Writer, errOut io.Writer, joinToken string) {
	s.executeCommand(
		stdOut,
		errOut,
		[]RoleGroup{
			{
				Roles: []types.NodeRole{types.NodeRoleEmpty},
				Handler: func(_ *types.Node, _ *APICommandsService, local *LocalCommandsService) (*commandstypes.ExecResponseDTO, error) {
					local.Join(stdOut, errOut, joinToken)
					return nil, nil
				},
			},
		},
	)
}
