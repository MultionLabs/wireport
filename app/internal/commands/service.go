package commands

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"wireport/cmd/server/config"
	commandstypes "wireport/internal/commands/types"
	"wireport/internal/dockerutils"
	"wireport/internal/encryption/mtls"
	"wireport/internal/joinrequests"
	joinrequeststypes "wireport/internal/joinrequests/types"
	"wireport/internal/networkapps"
	"wireport/internal/nodes"
	"wireport/internal/nodes/types"
	node_types "wireport/internal/nodes/types"
	"wireport/internal/publicservices"
	"wireport/internal/ssh"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct{}

// gateway

func (s *Service) GatewayStatus(creds *ssh.Credentials, stdOut io.Writer) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🔍 Checking wireport Gateway Status\n")
	fmt.Fprintf(stdOut, "================================\n\n")

	// SSH Connection Check
	fmt.Fprintf(stdOut, "📡 SSH Connection\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err := sshService.Connect(creds)
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	// Docker Installation Check
	fmt.Fprintf(stdOut, "🐳 Docker Installation\n")
	dockerInstalled, err := sshService.IsDockerInstalled()

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	var dockerVersion string

	if dockerInstalled {
		fmt.Fprintf(stdOut, "   Status: ✅ Installed\n")

		// Get Docker version
		dockerVersion, err = sshService.GetDockerVersion()
		if err == nil {
			fmt.Fprintf(stdOut, "   Version: %s\n", dockerVersion)
		}
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Installed\n\n")
		fmt.Fprintf(stdOut, "💡 Install Docker to continue with wireport setup.\n\n")
		return
	}

	// Docker Permissions Check
	fmt.Fprintf(stdOut, "   Permissions: ")
	dockerAccessible, err := sshService.IsDockerAccessible()

	if err != nil {
		fmt.Fprintf(stdOut, "❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if dockerAccessible {
		fmt.Fprintf(stdOut, "✅ User has access\n")
	} else {
		fmt.Fprintf(stdOut, "❌ User lacks permissions\n")
		fmt.Fprintf(stdOut, "💡 Add user to docker group.\n\n")
		return
	}
	fmt.Fprintf(stdOut, "\n")

	// wireport Status Check
	fmt.Fprintf(stdOut, "🚀 wireport Status\n")
	isRunning, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	var containerStatus string

	if isRunning {
		fmt.Fprintf(stdOut, "   Status: ✅ Running\n")

		// Get detailed container status
		containerStatus, err = sshService.GetWireportContainerStatus()
		if err == nil && containerStatus != "" {
			fmt.Fprintf(stdOut, "   Details: %s\n", containerStatus)
		}
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Running\n")

		// Check if container exists but is stopped
		containerStatus, err = sshService.GetWireportContainerStatus()
		if err == nil && containerStatus != "" {
			fmt.Fprintf(stdOut, "   Details: %s\n", containerStatus)
		}

		fmt.Fprintf(stdOut, "   💡 Run 'wireport gateway up %s@%s:%d' to install and start wireport gateway node.\n", creds.Username, creds.Host, creds.Port)
	}
	fmt.Fprintf(stdOut, "\n")

	// Docker Network Status Check
	fmt.Fprintf(stdOut, "🌐 wireport Docker Network\n")
	networkStatus, err := sshService.GetWireportNetworkStatus()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if networkStatus != "" {
		fmt.Fprintf(stdOut, "   Network: ✅ '%s' exists\n", strings.TrimSpace(networkStatus))
	} else {
		fmt.Fprintf(stdOut, "   Network: ❌ %s not found\n", config.Config.DockerNetworkName)
		fmt.Fprintf(stdOut, "💡 Network will be created when wireport starts.\n")
	}
	fmt.Fprintf(stdOut, "\n")

	fmt.Fprintf(stdOut, "✨ Status check completed successfully!\n")
}

func (s *Service) GatewayUp(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Gateway Up\n")
	fmt.Fprintf(stdOut, "==========================\n\n")

	// SSH Connection
	fmt.Fprintf(stdOut, "📡 Connecting to gateway...\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err := sshService.Connect(creds)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	// Check if already running
	fmt.Fprintf(stdOut, "🔍 Checking current status...\n")
	isRunning, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if isRunning {
		fmt.Fprintf(stdOut, "   Status: ✅ Already Running\n")
		fmt.Fprintf(stdOut, "   💡 wireport gateway container is already running on this gateway and bootstrapping is not required.\n\n")
		return
	}

	fmt.Fprintf(stdOut, "   Status: ❌ Not Running\n")
	fmt.Fprintf(stdOut, "   💡 Proceeding with installation...\n\n")

	// Installation
	fmt.Fprintf(stdOut, "📦 Installing wireport...\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, clientJoinToken, err := sshService.InstallWireportGateway()

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Installation Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Installation Completed\n\n")

	// Verification
	fmt.Fprintf(stdOut, "✅ Verifying installation...\n")
	installationConfirmed, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if installationConfirmed {
		fmt.Fprintf(stdOut, "   Status: ✅ Verified Successfully, Running\n")
		fmt.Fprintf(stdOut, "   🎉 wireport has been successfully installed and started on the gateway!\n\n")
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Verified Failed\n")
		fmt.Fprintf(stdOut, "   💡 wireport container was not found running after installation.\n\n")
	}

	if clientJoinToken != nil {
		fmt.Fprintf(stdOut, "   🔑 Applying Client Join Token: %s...\n", (*clientJoinToken)[:100])

		s.Join(nodesRepository, stdOut, errOut, *clientJoinToken)
	}

	fmt.Fprintf(stdOut, "✨ Bootstrap process completed!\n")
}

func (s *Service) GatewayDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	// First, try to determine if we are executing on a gateway node or locally
	currentNode, err := nodesRepository.GetCurrentNode()

	if err == nil && currentNode != nil && currentNode.Role == node_types.NodeRoleGateway {
		// Local execution – just detach and remove docker network like in ServerDown
		err = dockerutils.DetachDockerNetworkFromAllContainers()
		if err != nil {
			fmt.Fprintf(errOut, "Error detaching docker network: %v\n", err)
			return
		}

		err = dockerutils.RemoveDockerNetwork()
		if err != nil {
			fmt.Fprintf(errOut, "Error removing docker network: %v\n", err)
			return
		}

		return
	}

	// Remote execution – credentials are required
	if creds == nil {
		fmt.Fprintf(errOut, "Error: SSH credentials are required\n")
		return
	}

	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Gateway Teardown\n")
	fmt.Fprintf(stdOut, "=========================\n\n")

	// SSH Connection
	fmt.Fprintf(stdOut, "📡 Connecting to gateway...\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err = sshService.Connect(creds)
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	// Check if gateway is running
	isRunning, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if !isRunning {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Running\n")
		fmt.Fprintf(stdOut, "   💡 wireport gateway is not running\n\n")
		return
	}

	// Teardown wireport gateway
	fmt.Fprintf(stdOut, "🛑 Teardown wireport gateway...\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, err = sshService.TeardownWireportGateway()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Teardown Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Teardown Completed\n\n")

	fmt.Fprintf(stdOut, "✨ Gateway teardown process completed!\n")
}

func (s *Service) GatewayUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		sshService := ssh.NewService()

		err = sshService.Connect(creds)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to connect to gateway: %v\n", err)
			return
		}

		defer sshService.Close()

		fmt.Fprintf(stdOut, "Upgrading wireport gateway node...\n")

		success, err := sshService.UpgradeWireportGateway()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to upgrade wireport gateway: %v\n", err)
			return
		}

		if success {
			fmt.Fprintf(stdOut, "Wireport gateway node upgraded successfully\n")
		} else {
			fmt.Fprintf(errOut, "Failed to upgrade wireport gateway\n")
		}
	default:
		fmt.Fprintf(errOut, "Can only upgrade wireport gateway node from a client node\n")
		return
	}
}

func (s *Service) GatewayStart(gatewayPublicIP string, nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository,
	_ *gorm.DB, stdOut io.Writer, errOut io.Writer, gatewayStartConfigureOnly bool, router http.Handler) {
	gatewayNode, err := nodesRepository.EnsureGatewayNode(types.IPMarshable{
		IP: net.ParseIP(gatewayPublicIP),
	}, config.Config.WGPublicPort, gatewayPublicIP, config.Config.ControlServerPort)

	if err != nil {
		fmt.Fprintf(errOut, "wireport gateway node start failed: %v\n", err)
		fmt.Fprintf(errOut, "Failed to ensure gateway node: %v\n", err)
		return
	}

	if gatewayNode.GatewayCertBundle == nil {
		fmt.Fprintf(errOut, "wireport gateway node start failed: no gateway cert bundle found\n")
		return
	}

	serverError := make(chan error, 1)

	if !gatewayStartConfigureOnly {
		go func() {
			var tlsConfig *tls.Config

			tlsConfig, err = gatewayNode.GatewayCertBundle.GetServerTLSConfig()

			if err != nil {
				serverError <- fmt.Errorf("failed to get TLS config: %v", err)
				return
			}

			// Create TLS server
			server := &http.Server{
				Addr:      fmt.Sprintf(":%d", config.Config.ControlServerPort),
				Handler:   router,
				TLSConfig: tlsConfig,
			}

			if err = server.ListenAndServeTLS("", ""); err != nil {
				serverError <- err
			}
		}()
	}

	publicServices := publicServicesRepository.GetAll()

	err = gatewayNode.SaveConfigs(publicServices, true)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save configs: %v\n", err)
		return
	}

	if !gatewayStartConfigureOnly {
		fmt.Fprintf(stdOut, "wireport server has started with mTLS on gateway: %s\n", *gatewayNode.WGPublicIP)
	} else {
		fmt.Fprintf(stdOut, "wireport has been configured on the gateway: %s\n", *gatewayNode.WGPublicIP)
	}

	if !gatewayStartConfigureOnly {
		// Block on the server error channel
		if err := <-serverError; err != nil {
			fmt.Fprintf(errOut, "Server error: %v\n", err)
		}
	}
}

// client

func (s *Service) ClientNew(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, publicServicesRepository *publicservices.Repository,
	stdOut io.Writer, errOut io.Writer, joinRequestClientCreation bool, quietClientCreation bool, waitClientCreation bool) {

	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil || currentNode == nil {
		if !waitClientCreation {
			fmt.Fprintf(errOut, "Current node not found, skipping client creation\n")
			return
		}

		// wait for 10 seconds with retries every 1 second
		for range 10 {
			time.Sleep(1 * time.Second)

			currentNode, err = nodesRepository.GetCurrentNode()

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
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleGateway:
		totalWireguardClients, availableWireguardClients, err := nodesRepository.TotalAvailableWireguardClients()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to count available wireguard clients: %v\n", err)
			return
		}

		totalJoinRequests := joinRequestsRepository.CountAll()

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

			err = nodesRepository.SaveNode(currentNode)

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

			joinRequest, err = joinRequestsRepository.Create(joinRequestID, types.UDPAddrMarshable{
				UDPAddr: net.UDPAddr{
					IP:   net.ParseIP(*currentNode.WGPublicIP),
					Port: int(config.Config.ControlServerPort),
				},
			}, nil, types.NodeRoleClient, clientCertBundle)

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
				fmt.Fprintf(stdOut, "wireport:\n\nNew client created, use the following join request to connect to the network:\n\nwireport join %s\n", *joinRequestBase64)
			} else {
				fmt.Fprintf(stdOut, "%s\n", *joinRequestBase64)
			}
		} else {
			if !quietClientCreation {
				fmt.Fprintf(stdOut, "Join request flag not detected, creating client node without generating a join request\n")
			}

			var clientNode *node_types.Node

			clientNode, err = nodesRepository.CreateClient()

			if err != nil {
				fmt.Fprintf(errOut, "Failed to create client: %v\n", err)
				return
			}

			if !quietClientCreation {
				fmt.Fprintf(stdOut, "Client node created without join request\n")
			}

			// save configs & restart services
			publicServices := publicServicesRepository.GetAll()

			currentNode, err = nodesRepository.GetCurrentNode()

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
				fmt.Fprintf(stdOut, "New client created, use the following wireguard config on your client node to connect to the network:\n\n%s\n", *wireguardConfig)
			} else {
				fmt.Fprintf(stdOut, "%s\n", *wireguardConfig)
			}
		}
	}
}

func (s *Service) ClientList(nodesRepository *nodes.Repository, requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case node_types.NodeRoleClient:
		// remote execution
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case node_types.NodeRoleGateway:
		// local execution
		clientNodes, err := nodesRepository.GetNodesByRole(node_types.NodeRoleClient)

		if err != nil {
			fmt.Fprintf(errOut, "Error getting nodes: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "ID\tPRIVATE IP\n")

		for _, clientNode := range clientNodes {

			if requestFromNodeID != nil && clientNode.ID == *requestFromNodeID {
				fmt.Fprintf(stdOut, "%s*\t%s\n", clientNode.ID, clientNode.WGConfig.Interface.Address.String())
			} else {
				fmt.Fprintf(stdOut, "%s\t%s\n", clientNode.ID, clientNode.WGConfig.Interface.Address.String())
			}
		}

		return
	default:
		fmt.Fprintf(errOut, "Error: Current node is not a client or gateway\n")
		return
	}
}

// server

func (s *Service) ServerNew(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, stdOut io.Writer, errOut io.Writer, forceServerCreation bool, quietServerCreation bool, dockerSubnet string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		// remote execution
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleGateway:
		// local execution
		totalDockerSubnets, availableDockerSubnets, err := nodesRepository.TotalAndAvailableDockerSubnets()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to count available Docker subnets: %v\n", err)
			return
		}

		totalServerRoleJoinRequests := joinRequestsRepository.CountServerJoinRequests()

		if availableDockerSubnets <= 0 || totalServerRoleJoinRequests >= availableDockerSubnets {
			fmt.Fprintf(errOut, "No Docker subnets available. Please delete some server nodes (total used: %d) or server join-requests (total used: %d) to free up some subnets.\n", totalDockerSubnets, totalServerRoleJoinRequests)
			return
		}

		totalWireguardClients, availableWireguardClients, err := nodesRepository.TotalAvailableWireguardClients()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to count available Wireguard clients: %v\n", err)
			return
		}

		totalJoinRequests := joinRequestsRepository.CountAll()

		if availableWireguardClients <= 0 || totalJoinRequests >= availableWireguardClients {
			fmt.Fprintf(errOut, "No Wireguard clients available. Please delete some client/server nodes (total used: %d) or client/server join-requests (total used: %d) to free up some clients.\n", totalWireguardClients, totalJoinRequests)
			return
		}

		var dockerSubnetPtr *string
		var parsedDockerSubnet *types.IPNetMarshable

		if dockerSubnet != "" {
			// validate the subnet format
			parsedDockerSubnet, err = types.ParseIPNetMarshable(dockerSubnet, true)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to parse Docker subnet: %v\n", err)
				return
			}

			if !nodesRepository.IsDockerSubnetAvailable(parsedDockerSubnet) {
				fmt.Fprintf(errOut, "Docker subnet %s is already in use\n", dockerSubnet)
				return
			}

			dockerSubnetPtr = &dockerSubnet

			if !quietServerCreation {
				fmt.Fprintf(stdOut, "Using custom Docker subnet: %s\n", dockerSubnet)
			}
		}

		if forceServerCreation {
			if !quietServerCreation {
				fmt.Fprintf(stdOut, "Force flag detected, creating server node without generating a join request\n")
			}

			_, err = nodesRepository.CreateServer(dockerSubnetPtr)

			if err != nil {
				fmt.Fprintf(errOut, "Failed to create server node: %v\n", err)
				return
			}

			if !quietServerCreation {
				fmt.Fprintf(stdOut, "Server node created without join request\n")
			}

			return
		}

		gatewayNode, err := nodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to get gateway node: %v\n", err)
			return
		}

		joinRequestID := uuid.New().String()

		err = gatewayNode.GatewayCertBundle.AddClient(mtls.Options{
			CommonName: joinRequestID,
			Expiry:     config.Config.CertExpiry,
		})

		if err != nil {
			fmt.Fprintf(errOut, "Failed to add client to gateway cert bundle: %v\n", err)
			return
		}

		err = nodesRepository.SaveNode(gatewayNode)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to save gateway node: %v\n", err)
			return
		}

		clientCertBundle, err := gatewayNode.GatewayCertBundle.GetClientBundlePublic(joinRequestID)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to get client cert bundle: %v\n", err)
			return
		}

		joinRequest, err := joinRequestsRepository.Create(joinRequestID, types.UDPAddrMarshable{
			UDPAddr: net.UDPAddr{
				IP:   net.ParseIP(*gatewayNode.WGPublicIP),
				Port: int(config.Config.ControlServerPort),
			},
		}, dockerSubnetPtr, types.NodeRoleServer, clientCertBundle)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to create join request: %v\n", err)
			return
		}

		joinRequestBase64, err := joinRequest.ToBase64()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to encode join request: %v\n", err)
			return
		}

		if !quietServerCreation {
			fmt.Fprintf(stdOut, "wireport:\n\nServer created, execute the command below on the server to join the network:\n\nwireport join %s\n", *joinRequestBase64)
		} else {
			fmt.Fprintf(stdOut, "%s\n", *joinRequestBase64)
		}
	}
}

func (s *Service) ServerStart(nodesRepository *nodes.Repository, stdOut io.Writer, errOut io.Writer) {
	fmt.Fprintf(stdOut, "Starting wireport server\n")

	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
		return
	}

	if currentNode == nil {
		fmt.Fprintf(errOut, "No current node found\n")
		return
	}

	if currentNode.Role != types.NodeRoleServer {
		fmt.Fprintf(errOut, "Current node is not a server node\n")
		return
	}

	publicServices := []*publicservices.PublicService{}

	err = currentNode.SaveConfigs(publicServices, true)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save server node configs: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Server node configs saved to the disk successfully\n")

	for {
		fmt.Fprintf(stdOut, "Ensuring docker network is attached to all containers\n")

		err = dockerutils.EnsureDockerNetworkIsAttachedToAllContainers()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to ensure docker network is attached to all containers: %v\n", err)
		}

		time.Sleep(time.Second * 30)
	}
}

func (s *Service) ServerList(nodesRepository *nodes.Repository, requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case node_types.NodeRoleClient:
		// remote execution
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case node_types.NodeRoleGateway:
		// local execution
		serverNodes, err := nodesRepository.GetNodesByRole(node_types.NodeRoleServer)

		if err != nil {
			fmt.Fprintf(errOut, "Error getting nodes: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "ID\tPRIVATE IP\n")

		for _, serverNode := range serverNodes {

			if requestFromNodeID != nil && serverNode.ID == *requestFromNodeID {
				fmt.Fprintf(stdOut, "%s*\t%s\n", serverNode.ID, serverNode.WGConfig.Interface.Address.String())
			} else {
				fmt.Fprintf(stdOut, "%s\t%s\n", serverNode.ID, serverNode.WGConfig.Interface.Address.String())
			}
		}

		return
	default:
		fmt.Fprintf(errOut, "Error: Current node is not a client or gateway\n")
		return
	}
}

func (s *Service) ServerUpgrade(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, nodesRepository *nodes.Repository) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		sshService := ssh.NewService()

		err = sshService.Connect(creds)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to connect to server: %v\n", err)
			return
		}

		defer sshService.Close()

		fmt.Fprintf(stdOut, "Upgrading wireport server node...\n")

		success, err := sshService.UpgradeWireportServer()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to upgrade wireport server: %v\n", err)
			return
		}

		if success {
			fmt.Fprintf(stdOut, "Wireport server node upgraded successfully\n")
		} else {
			fmt.Fprintf(errOut, "Failed to upgrade wireport server\n")
		}
	default:
		fmt.Fprintf(errOut, "Can only upgrade wireport server node from a client node\n")
		return
	}
}

func (s *Service) ServerStatus(creds *ssh.Credentials, stdOut io.Writer) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🔍 Checking wireport Server Status\n")
	fmt.Fprintf(stdOut, "==================================\n\n")

	// SSH Connection Check
	fmt.Fprintf(stdOut, "📡 SSH Connection\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err := sshService.Connect(creds)
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	// Docker Installation Check
	fmt.Fprintf(stdOut, "🐳 Docker Installation\n")
	dockerInstalled, err := sshService.IsDockerInstalled()

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	var dockerVersion string

	if dockerInstalled {
		fmt.Fprintf(stdOut, "   Status: ✅ Installed\n")

		// Get Docker version
		dockerVersion, err = sshService.GetDockerVersion()
		if err == nil {
			fmt.Fprintf(stdOut, "   Version: %s\n", dockerVersion)
		}
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Installed\n\n")
		fmt.Fprintf(stdOut, "💡 Install Docker to continue with wireport setup.\n\n")
		return
	}

	// Docker Permissions Check
	fmt.Fprintf(stdOut, "   Permissions: ")
	dockerAccessible, err := sshService.IsDockerAccessible()

	if err != nil {
		fmt.Fprintf(stdOut, "❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if dockerAccessible {
		fmt.Fprintf(stdOut, "✅ User has access\n")
	} else {
		fmt.Fprintf(stdOut, "❌ User lacks permissions\n")
		fmt.Fprintf(stdOut, "💡 Add user to docker group.\n\n")
		return
	}
	fmt.Fprintf(stdOut, "\n")

	// wireport Server Status Check
	fmt.Fprintf(stdOut, "🚀 wireport Server Status\n")
	isRunning, err := sshService.IsWireportServerContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	var containerStatus string

	if isRunning {
		fmt.Fprintf(stdOut, "   Status: ✅ Running\n")

		// Get detailed container status
		containerStatus, err = sshService.GetWireportServerContainerStatus()
		if err == nil && containerStatus != "" {
			fmt.Fprintf(stdOut, "   Details: %s\n", containerStatus)
		}
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Running\n")

		// Check if container exists but is stopped
		containerStatus, err = sshService.GetWireportServerContainerStatus()
		if err == nil && containerStatus != "" {
			fmt.Fprintf(stdOut, "   Details: %s\n", containerStatus)
		}

		fmt.Fprintf(stdOut, "   💡 Run 'wireport server up %s@%s:%d' to install and start wireport server node.\n", creds.Username, creds.Host, creds.Port)
	}
	fmt.Fprintf(stdOut, "\n")

	// Docker Network Status Check
	fmt.Fprintf(stdOut, "🌐 wireport Docker Network\n")
	networkStatus, err := sshService.GetWireportNetworkStatus()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if networkStatus != "" {
		fmt.Fprintf(stdOut, "   Network: ✅ '%s' exists\n", strings.TrimSpace(networkStatus))
	} else {
		fmt.Fprintf(stdOut, "   Network: ❌ %s not found\n", config.Config.DockerNetworkName)
		fmt.Fprintf(stdOut, "💡 Network will be created when wireport server starts.\n")
	}
	fmt.Fprintf(stdOut, "\n")

	fmt.Fprintf(stdOut, "✨ Status check completed successfully!\n")
}

func (s *Service) ServerUp(nodesRepository *nodes.Repository, joinRequestsRepository *joinrequests.Repository, creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer, dockerSubnet string) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Server Connect\n")
	fmt.Fprintf(stdOut, "=========================\n\n")

	// SSH Connection
	fmt.Fprintf(stdOut, "📡 Connecting to server...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err := sshService.Connect(creds)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	stdOutWriter := bytes.NewBufferString("")
	errOutWriter := bytes.NewBufferString("")

	s.ServerNew(nodesRepository, joinRequestsRepository, stdOutWriter, errOutWriter, false, true, dockerSubnet)

	if len(errOutWriter.String()) > 0 || len(stdOutWriter.String()) == 0 {
		fmt.Fprintf(errOut, "%s\n", errOutWriter.String())
		fmt.Fprintf(stdOut, "%s\n", stdOutWriter.String())
		fmt.Fprintf(stdOut, "❌ Failed to connect wireport server to the network\n")
		return
	}

	serverJoinToken := stdOutWriter.String()

	// Connection
	fmt.Fprintf(stdOut, "📦 Connecting wireport server to the network...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, err = sshService.InstallWireportServer(serverJoinToken)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Connection Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Connection Completed\n\n")

	// Verification
	fmt.Fprintf(stdOut, "✅ Verifying connection...\n")
	installationConfirmed, err := sshService.IsWireportServerContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if installationConfirmed {
		fmt.Fprintf(stdOut, "   Status: ✅ Verified Successfully, Running\n")
		fmt.Fprintf(stdOut, "   🎉 wireport server has been successfully connected to the network!\n\n")
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   💡 wireport server container was not found running after connection.\n\n")
	}

	fmt.Fprintf(stdOut, "✨ Server connection process completed!\n")
}

func (s *Service) ServerDown(nodesRepository *nodes.Repository, creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	if currentNode.Role == node_types.NodeRoleServer {
		err = dockerutils.DetachDockerNetworkFromAllContainers()

		if err != nil {
			fmt.Fprintf(errOut, "Error detaching docker network: %v\n", err)
			return
		}

		err = dockerutils.RemoveDockerNetwork()

		if err != nil {
			fmt.Fprintf(errOut, "Error removing docker network: %v\n", err)
			return
		}

		return
	}

	if creds == nil {
		fmt.Fprintf(errOut, "Error: SSH credentials are required\n")
		return
	}

	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Server Teardown\n")
	fmt.Fprintf(stdOut, "==========================\n\n")

	// SSH Connection
	fmt.Fprintf(stdOut, "📡 Connecting to server...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err = sshService.Connect(creds)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()
	fmt.Fprintf(stdOut, "   Status: ✅ Connected\n\n")

	// Check if server is running
	isRunning, err := sshService.IsWireportServerContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if !isRunning {
		fmt.Fprintf(stdOut, "   Status: ❌ Not Running\n")
		fmt.Fprintf(stdOut, "   💡 wireport server is not running\n\n")
		return
	}

	// Teardown wireport server
	fmt.Fprintf(stdOut, "🛑 Teardown wireport server...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, err = sshService.TeardownWireportServer()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Teardown Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Teardown Completed\n\n")

	fmt.Fprintf(stdOut, "✨ Server teardown process completed!\n")
}

// service

func (s *Service) ServicePublish(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer,
	localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot publish services\n")
		return
	}

	err = publicServicesRepository.Save(&publicservices.PublicService{
		LocalProtocol:  localProtocol,
		LocalHost:      localHost,
		LocalPort:      localPort,
		PublicProtocol: publicProtocol,
		PublicHost:     publicHost,
		PublicPort:     publicPort,
	})

	if err != nil {
		fmt.Fprintf(errOut, "Error creating public service: %v\n", err)
		return
	}

	gatewayNode, err := nodesRepository.GetGatewayNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
		return
	}

	err = gatewayNode.SaveConfigs(publicServicesRepository.GetAll(), false)

	if err != nil {
		fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
		return
	}

	err = networkapps.RestartNetworkApps(false, false, true)

	if err != nil {
		fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Service %s://%s:%d is now published on\n\n\t\t%s://%s:%d\n\n", localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
}

func (s *Service) ServiceUnpublish(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot publish services\n")
		return
	}

	unpublished := publicServicesRepository.Delete(publicProtocol, publicHost, publicPort)

	if unpublished {
		gatewayNode, err := nodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServicesRepository.GetAll(), false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "\nService %s://%s:%d is now unpublished\n", publicProtocol, publicHost, publicPort)
	} else {
		fmt.Fprintf(stdOut, "\nService %s://%s:%d was not found or was already unpublished\n", publicProtocol, publicHost, publicPort)
	}
}

func (s *Service) ServiceList(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot list services\n")
		return
	}

	services := publicServicesRepository.GetAll()

	fmt.Fprintf(stdOut, "PUBLIC\tLOCAL\n")

	for _, service := range services {
		fmt.Fprintf(stdOut, "%s://%s:%d\t%s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort, service.LocalProtocol, service.LocalHost, service.LocalPort)
	}
}

// service params

func (s *Service) ServiceParamNew(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot add service params\n")
		return
	}

	updated := publicServicesRepository.AddParam(publicProtocol, publicHost, publicPort, paramType, paramValue)

	if updated {
		gatewayNode, err := nodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServicesRepository.GetAll(), false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "Parameter added successfully\n")
	} else {
		fmt.Fprintf(stdOut, "Parameter was not added (probably already exists)\n")
	}
}

func (s *Service) ServiceParamRemove(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot remove service params\n")
		return
	}

	removed := publicServicesRepository.RemoveParam(publicProtocol, publicHost, publicPort, paramType, paramValue)

	if removed {
		gatewayNode, err := nodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServicesRepository.GetAll(), false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "Parameter removed successfully\n")
	} else {
		fmt.Fprintf(stdOut, "Parameter was not removed (probably not found)\n")
	}
}

func (s *Service) ServiceParamList(nodesRepository *nodes.Repository, publicServicesRepository *publicservices.Repository, stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	currentNode, err := nodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleClient:
		apiService := APIService{
			Host:             currentNode.GatewayPublicIP,
			Port:             currentNode.GatewayPublicPort,
			ClientCertBundle: currentNode.ClientCertBundle,
		}

		var execResponseDTO commandstypes.ExecResponseDTO

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
	case types.NodeRoleServer:
		fmt.Fprintf(errOut, "Server node cannot list service params\n")
		return
	}

	service, err := publicServicesRepository.Get(publicProtocol, publicHost, publicPort)

	if err != nil {
		fmt.Fprintf(errOut, "Error getting service: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Params of %s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort)

	for _, param := range service.Params {
		fmt.Fprintf(stdOut, "%s\n", param.ParamValue)
	}

	fmt.Fprintf(stdOut, "\n")
}

// join

func (s *Service) Join(nodesRepository *nodes.Repository, stdOut io.Writer, errOut io.Writer, joinToken string) {
	joinRequest := &joinrequeststypes.JoinRequest{}

	err := joinRequest.FromBase64(joinToken)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to parse join request: %v\n", err)
		return
	}

	joinRequestsService := joinrequests.NewAPIService(&joinRequest.ClientCertBundle)

	response, err := joinRequestsService.Join(joinToken, joinRequest)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to join network: %v\n", err)
		return
	}

	currentNode := response.NodeConfig

	if currentNode == nil {
		fmt.Fprintf(errOut, "Failed to get node config from join response\n")
		return
	}

	currentNode.IsCurrentNode = true

	err = nodesRepository.SaveNode(currentNode)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save node config: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleServer:
		fmt.Fprintf(stdOut, "Setting up server node (configs and network)\n")

		if currentNode.DockerSubnet == nil {
			fmt.Fprintf(errOut, "Failed to get docker subnet from node config\n")
			return
		}

		dockerSubnet, err := types.ParseIPNetMarshable(currentNode.DockerSubnet.String(), true)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to parse docker subnet: %v\n", err)
			return
		}

		err = dockerutils.EnsureDockerNetworkExistsAndAttached(dockerSubnet)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to ensure docker network %s with subnet %s exists and is attached: %v\n", config.Config.DockerNetworkName, dockerSubnet.String(), err)
			return
		}

		publicServices := []*publicservices.PublicService{}

		err = currentNode.SaveConfigs(publicServices, true)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to save node configs: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "Successfully saved node config to the database\n")
	case types.NodeRoleClient:
		wireguardConfig, err := currentNode.GetFormattedWireguardConfig()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to get wireguard config: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "New client created, use the following wireguard config to connect to the network:\n\n%s\n", *wireguardConfig)
	default:
		fmt.Fprintf(errOut, "Invalid node role: %s\n", currentNode.Role)
		return
	}

	fmt.Fprintf(stdOut, "Successfully joined the network\n")
}
