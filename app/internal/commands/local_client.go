package commands

import (
	"fmt"
	"io"
	"strings"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/encryption/mtls"
	joinrequeststypes "wireport/internal/joinrequests/types"
	"wireport/internal/networkapps"
	"wireport/internal/nodes/types"

	"github.com/google/uuid"
)

func (s *LocalCommandsService) ClientNew(stdOut io.Writer, errOut io.Writer, joinRequestClientCreation bool, quietClientCreation bool, waitClientCreation bool) {
	currentNode, err := s.NodesRepository.GetCurrentNode()

	if err != nil || currentNode == nil {
		if !waitClientCreation {
			fmt.Fprintf(errOut, "Current node not found, skipping client creation\n")
			return
		}

		// wait for 10 seconds with retries every 1 second
		for range 10 {
			time.Sleep(1 * time.Second)

			currentNode, err = s.NodesRepository.GetCurrentNode()

			if err == nil && currentNode != nil {
				if !quietClientCreation {
					fmt.Fprintf(stdOut, "Current node found, creating client\n")
				}

				break
			}
		}

		if err != nil || currentNode == nil {
			fmt.Fprintf(errOut, "Failed to get current node after waiting and multiple retries: %v\n", err)
			return
		}
	}

	switch currentNode.Role {
	case types.NodeRoleGateway:
		totalWireguardClients, availableWireguardClients, err := s.NodesRepository.TotalAvailableWireguardClients()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to count available wireguard clients: %v\n", err)
			return
		}

		totalJoinRequests := s.JoinRequestsRepository.CountAll()

		if availableWireguardClients <= 0 || totalJoinRequests >= availableWireguardClients {
			fmt.Fprintf(errOut, "No available wireguard client slots. Please delete some client/server nodes (total used: %d) or client/server join-requests (total used: %d) to free up some wireguard client slots.\n", totalWireguardClients, totalJoinRequests)
			return
		}

		if joinRequestClientCreation {
			// create join request
			joinRequestID := uuid.New().String()

			err = currentNode.GatewayCertBundle.AddClient(mtls.Options{
				CommonName: joinRequestID,
				Expiry:     config.Config.CertExpiry,
			})

			if err != nil {
				fmt.Fprintf(errOut, "Failed to add client to gateway cert bundle: %v\n", err)
				return
			}

			err = s.NodesRepository.SaveNode(currentNode)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to save gateway node: %v\n", err)
				return
			}

			var clientCertBundle *mtls.FullClientBundle
			clientCertBundle, err = currentNode.GatewayCertBundle.GetClientBundlePublic(joinRequestID)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to get client cert bundle: %v\n", err)
				return
			}

			var joinRequest *joinrequeststypes.JoinRequest

			joinRequest, err = s.JoinRequestsRepository.Create(joinRequestID, *currentNode.WGPublicIP, config.Config.ControlServerPort, nil, types.NodeRoleClient, clientCertBundle)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to create join request: %v\n", err)
				return
			}

			var joinRequestBase64 *string

			joinRequestBase64, err = joinRequest.ToBase64()

			if err != nil {
				fmt.Fprintf(errOut, "Failed to encode join request: %v\n", err)
				return
			}

			if !quietClientCreation {
				fmt.Fprintf(stdOut, "New client created, use the following command to connect to the network:\n\nwireport join %s\n", *joinRequestBase64)
			} else {
				fmt.Fprintf(stdOut, "%s\n", *joinRequestBase64)
			}
		} else {
			var clientNode *types.Node

			clientNode, err = s.NodesRepository.CreateClient()

			if err != nil {
				fmt.Fprintf(errOut, "Failed to create client: %v\n", err)
				return
			}

			// save configs & restart services
			publicServices, err := s.PublicServicesRepository.GetAll()

			if err != nil {
				fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
				return
			}

			currentNode, err = s.NodesRepository.GetCurrentNode()

			if err != nil {
				fmt.Fprintf(errOut, "Failed to get a fresh current node instance after creating client: %v\n", err)
				return
			}

			err = currentNode.SaveConfigs(publicServices, false)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to save gateway configs: %v\n", err)
				return
			}

			err = networkapps.RestartNetworkApps(true, false, false)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to restart services: %v\n", err)
			}

			wireguardConfig, _ := clientNode.GetFormattedWireguardConfig()

			if !quietClientCreation {
				fmt.Fprintf(stdOut, "# New wireport client has been successfully created!\n")
				fmt.Fprintf(stdOut, "# Use the WireGuard config below on your client device to connect to the wireport network:\n\n")
				fmt.Fprintf(stdOut, "%s\n", *wireguardConfig)
			} else {
				fmt.Fprintf(stdOut, "%s\n", *wireguardConfig)
			}
		}
	}
}

func (s *LocalCommandsService) ClientList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	clientNodes, err := s.NodesRepository.GetNodesByRole(types.NodeRoleClient)

	if err != nil {
		fmt.Fprintf(errOut, "Error getting nodes: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "CLIENT PRIVATE IP\n")
	fmt.Fprintf(stdOut, "%s\n", strings.Repeat("=", 80))

	if len(clientNodes) > 0 {
		for _, clientNode := range clientNodes {
			if requestFromNodeID != nil && clientNode.ID == *requestFromNodeID {
				fmt.Fprintf(stdOut, "%s*\n", clientNode.WGConfig.Interface.Address.String())
			} else {
				fmt.Fprintf(stdOut, "%s\n", clientNode.WGConfig.Interface.Address.String())
			}
		}
	} else {
		fmt.Fprintf(stdOut, "No clients are registered on this gateway\n")
	}
}
