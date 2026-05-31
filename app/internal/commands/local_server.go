package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/dockerutils"
	"wireport/internal/encryption/mtls"
	"wireport/internal/joinrequests"
	"wireport/internal/publicservices"
	"wireport/internal/ssh"
	"wireport/internal/utils"

	"github.com/google/uuid"

	"wireport/internal/nodes/types"

	"gorm.io/gorm"
)

// Label keys expected in Docker Compose (or equivalent) to declare a wireport publication.
const (
	wireportServiceLocalLabel  = "wireport.service.local"
	wireportServicePublicLabel = "wireport.service.public"
)

func (s *LocalCommandsService) ServerNew(forceServerCreation bool, quietServerCreation bool, dockerSubnet string, stdOut io.Writer, errOut io.Writer) {
	totalDockerSubnets, availableDockerSubnets, err := s.NodesRepository.TotalAndAvailableDockerSubnets()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to count available Docker subnets: %v\n", err)
		return
	}

	totalServerRoleJoinRequests := s.JoinRequestsRepository.CountServerJoinRequests()

	if availableDockerSubnets <= 0 || totalServerRoleJoinRequests >= availableDockerSubnets {
		fmt.Fprintf(errOut, "No Docker subnets available. Please delete some server nodes (total used: %d) or server join-requests (total used: %d) to free up some subnets.\n", totalDockerSubnets, totalServerRoleJoinRequests)
		return
	}

	totalWireguardClients, availableWireguardClients, err := s.NodesRepository.TotalAvailableWireguardClients()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to count available WireGuard clients: %v\n", err)
		return
	}

	totalJoinRequests := s.JoinRequestsRepository.CountAll()

	if availableWireguardClients <= 0 || totalJoinRequests >= availableWireguardClients {
		fmt.Fprintf(errOut, "No WireGuard clients available. Please delete some client/server nodes (total used: %d) or client/server join-requests (total used: %d) to free up some clients.\n", totalWireguardClients, totalJoinRequests)
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

		if !s.NodesRepository.IsDockerSubnetAvailable(parsedDockerSubnet) {
			fmt.Fprintf(errOut, "Docker subnet %s is already in use\n", dockerSubnet)
			return
		}

		dockerSubnetPtr = &dockerSubnet

		if !quietServerCreation {
			fmt.Fprintf(stdOut, "Using custom Docker subnet: %s\n", dockerSubnet)
		}
	}

	if forceServerCreation {
		_, err = s.NodesRepository.CreateServer(dockerSubnetPtr)

		if err != nil {
			fmt.Fprintf(errOut, "Failed to create server node: %v\n", err)
			return
		}

		if !quietServerCreation {
			fmt.Fprintf(stdOut, "Server node created\n")
		}

		return
	}

	gatewayNode, err := s.NodesRepository.GetGatewayNode()

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

	err = s.NodesRepository.SaveNode(gatewayNode)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save gateway node: %v\n", err)
		return
	}

	clientCertBundle, err := gatewayNode.GatewayCertBundle.GetClientBundlePublic(joinRequestID)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get client cert bundle: %v\n", err)
		return
	}

	joinRequest, err := s.JoinRequestsRepository.Create(joinRequestID, *gatewayNode.WGPublicIP, config.Config.ControlServerPort, dockerSubnetPtr, types.NodeRoleServer, clientCertBundle)

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
		fmt.Fprintf(stdOut, "✅ Server created, execute the command below on the server to join the network:\n\nwireport join %s\n", *joinRequestBase64)
	} else {
		fmt.Fprintf(stdOut, "%s\n", *joinRequestBase64)
	}
}

func (s *LocalCommandsService) ServerRemove(stdOut io.Writer, errOut io.Writer, serverNodeID string) {
	err := s.NodesRepository.DeleteServer(serverNodeID)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to remove server node '%s': %v\n", serverNodeID, err)
		return
	}

	fmt.Fprintf(stdOut, "Server node '%s' removed successfully\n", serverNodeID)
}

func (s *LocalCommandsService) ServerStart(apiCommandsService *APICommandsService, stdOut io.Writer, errOut io.Writer) {
	fmt.Fprintf(stdOut, "Starting wireport server\n")

	currentNode, err := s.NodesRepository.GetCurrentNode()

	if err != nil || currentNode == nil {
		fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
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
		currentNode, err = s.NodesRepository.GetCurrentNode()

		if err != nil || currentNode == nil {
			fmt.Fprintf(errOut, "Failed to get current node: %v\n", err)
			time.Sleep(time.Second * 30)
			continue
		}

		ensureDockerNetworkIsAttachedToAllContainers(stdOut, errOut)
		reconcileGatewayServicesWithDockerLabels(apiCommandsService, currentNode, stdOut, errOut)
		refreshNodeConfig(s, apiCommandsService, currentNode, stdOut, errOut)

		time.Sleep(time.Second * 30)
	}
}

func refreshNodeConfig(localCommandsService *LocalCommandsService, apiCommandsService *APICommandsService, currentNode *types.Node, stdOut io.Writer, errOut io.Writer) {
	nodeCommandResponse, err := apiCommandsService.NodeConfig()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get node config: %v\n", err)
		return
	}

	nodeConfig := nodeCommandResponse.NodeConfig

	if nodeConfig == nil {
		fmt.Fprintf(errOut, "Failed to get node config: %v\n", err)
		return
	}

	if nodeConfig.ID != currentNode.ID {
		fmt.Fprintf(errOut, "Node config mismatch: %s != %s\n", nodeConfig.ID, currentNode.ID)
		return
	}

	// for now we only update the labels
	// other fields can be resynced the same way if needed
	err = localCommandsService.NodesRepository.UpdateLabels(nodeConfig.ID, nodeConfig.Labels)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to update node config: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Node config updated successfully\n")
}

func ensureDockerNetworkIsAttachedToAllContainers(stdOut io.Writer, errOut io.Writer) {
	fmt.Fprintf(stdOut, "Ensuring docker network is attached to all containers\n")

	err := dockerutils.EnsureDockerNetworkIsAttachedToAllContainers()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to ensure docker network is attached to all containers: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Docker network is attached to all containers\n")
}

func filterServicesPublishedByNode(all []*publicservices.PublicService, nodeID string) []*publicservices.PublicService {
	out := make([]*publicservices.PublicService, 0)
	for _, svc := range all {
		if svc.PublishedByNodeID != nil && *svc.PublishedByNodeID == nodeID {
			out = append(out, svc)
		}
	}
	return out
}

// reconcileGatewayServicesWithDockerLabels syncs gateway publications for this node with
// Docker container labels. Unpublish/publish calls are batched at the end.
func reconcileGatewayServicesWithDockerLabels(api *APICommandsService, currentNode *types.Node, stdOut, errOut io.Writer) {
	serviceList, err := api.ServiceList()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to get services: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Retrieved %d services from gateway node\n", len(serviceList.Services))

	labelsByContainerName, err := dockerutils.ListAllContainerLabels()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to list all container labels: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Retrieved labels for %d containers\n", len(labelsByContainerName))

	// Some labels define services published on the gateway node.
	// Here we compare the list of actually published services with the list of services defined by labels:
	// if there are services defined by labels but not published, we publish them;
	// if there are services published but not defined by labels, we unpublish them.
	// We only manage services that are published by the current server node.

	// All services published by the current server node on the gateway — we only reconcile these.
	publishedHere := filterServicesPublishedByNode(serviceList.Services, currentNode.ID)

	fmt.Fprintf(stdOut, "Retrieved %d services published by current node\n", len(publishedHere))

	// Services still published by this node but no longer backed by valid labels → unpublish.
	servicesToUnpublish := make([]*publicservices.PublicService, 0, len(publishedHere))

	for _, service := range publishedHere {
		labels, ok := labelsByContainerName[service.LocalHost]
		missingLabels := !ok || labels[wireportServiceLocalLabel] == "" || labels[wireportServicePublicLabel] == ""
		if !missingLabels {
			continue
		}

		fmt.Fprintf(stdOut, "Service %s://%s:%d is published by the node, but labels are not set anymore - to be unpublished\n", service.LocalProtocol, service.LocalHost, service.LocalPort)
		servicesToUnpublish = append(servicesToUnpublish, service)
	}

	// Labels on containers that are not yet published by this node → candidates to publish.
	servicesToPublish := make([]*publicservices.PublicService, 0)

	for containerName, containerLabels := range labelsByContainerName {
		// If local host already matches a service published by this node, skip.
		alreadyPublished := false
		for _, svc := range publishedHere {
			if svc.LocalHost == containerName {
				alreadyPublished = true
				break
			}
		}
		if alreadyPublished {
			continue
		}

		// Not published by this node yet; validate label-defined addresses and build the publication.
		localAddress := containerLabels[wireportServiceLocalLabel]
		publicAddress := containerLabels[wireportServicePublicLabel]
		if localAddress == "" || publicAddress == "" {
			continue
		}

		localProtocol, localHost, localPort, err := utils.ParseAddress(localAddress)
		if err != nil {
			fmt.Fprintf(errOut, "Can not publish service %s: failed to parse local address: %v\n", containerName, err)
			continue
		}

		if *localHost != containerName {
			fmt.Fprintf(stdOut, "Can not publish service %s: local host %s does not match container name %s - the service definition is not valid, skipping\n", containerName, *localHost, containerName)
			continue
		}

		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(publicAddress)
		if err != nil {
			fmt.Fprintf(errOut, "Can not publish service %s: failed to parse public address: %v\n", containerName, err)
			continue
		}

		servicesToPublish = append(servicesToPublish, &publicservices.PublicService{
			PublishedByNodeID: &currentNode.ID,
			LocalHost:         *localHost,
			LocalProtocol:     *localProtocol,
			LocalPort:         *localPort,
			PublicProtocol:    *publicProtocol,
			PublicHost:        *publicHost,
			PublicPort:        *publicPort,
		})
	}

	// Apply unpublish then publish on the gateway.
	for _, service := range servicesToUnpublish {
		_, err := api.ServiceUnpublish(service.PublicProtocol, service.PublicHost, service.PublicPort)
		if err != nil {
			fmt.Fprintf(errOut, "Failed to unpublish service %s://%s:%d: %v\n", service.PublicProtocol, service.PublicHost, service.PublicPort, err)
			continue
		}

		fmt.Fprintf(stdOut, "Unpublished service %s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort)
	}

	for _, service := range servicesToPublish {
		publicationResult, err := api.ServicePublish(service.LocalProtocol, service.LocalHost, service.LocalPort, service.PublicProtocol, service.PublicHost, service.PublicPort)
		if err != nil || publicationResult.Stderr != "" {
			fmt.Fprintf(errOut, "Failed to publish service %s://%s:%d: -> %s://%s:%d: %v\n", service.LocalProtocol, service.LocalHost, service.LocalPort, service.PublicProtocol, service.PublicHost, service.PublicPort, err)
			continue
		}

		fmt.Fprintf(stdOut, "Published service %s://%s:%d -> %s://%s:%d\n", service.LocalProtocol, service.LocalHost, service.LocalPort, service.PublicProtocol, service.PublicHost, service.PublicPort)
	}
}

func (s *LocalCommandsService) ServerStatus(creds *ssh.Credentials, stdOut io.Writer) {
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

		fmt.Fprintf(stdOut, "   💡 Run 'wireport server up %s@%s:%d' to bootstrap wireport server and start it.\n", creds.Username, creds.Host, creds.Port)
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

	fmt.Fprintf(stdOut, "✨ Server Status check completed successfully!\n")
}

func (s *LocalCommandsService) ServerUp(creds *ssh.Credentials, image string, imageTag string, stdOut io.Writer, errOut io.Writer, dockerSubnet string, commandsService *Service) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Server Bootstrapping\n")
	fmt.Fprintf(stdOut, "================================\n\n")

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

	commandsService.ServerNew(stdOutWriter, errOutWriter, false, true, dockerSubnet)

	if len(errOutWriter.String()) > 0 || len(stdOutWriter.String()) == 0 {
		fmt.Fprintf(errOut, "%s\n", errOutWriter.String())
		fmt.Fprintf(stdOut, "%s\n", stdOutWriter.String())
		fmt.Fprintf(stdOut, "❌ Failed to connect server node to the wireport network\n")
		return
	}

	serverJoinToken := stdOutWriter.String()

	// Connection
	fmt.Fprintf(stdOut, "📦 Installing wireport server...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, err = sshService.InstallWireportServer(serverJoinToken, image, imageTag)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Connection Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Connection Completed\n\n")

	// Verification
	fmt.Fprintf(stdOut, "✅ Verifying installation...\n")
	installationConfirmed, err := sshService.IsWireportServerContainerRunning()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if !installationConfirmed {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   💡 Server node container was not found running after installation. Check logs on the server: docker logs %s\n\n", config.Config.WireportServerContainerName)
		return
	}

	fmt.Fprintf(stdOut, "   Container: ✅ Running\n")
	fmt.Fprintf(stdOut, "   Waiting for server to join the wireport network (up to 3 minutes)...\n")
	fmt.Fprintf(stdOut, "   💡 If this takes too long:\n")
	fmt.Fprintf(stdOut, "      - Check firewalls on the server host (outbound) AND the gateway host (inbound)\n")
	fmt.Fprintf(stdOut, "      - Firewall setup details: %s\n", config.Config.DocumentationURL)
	fmt.Fprintf(stdOut, "      - If firewalls look correct, inspect server container logs: docker logs -f %s\n\n", config.Config.WireportServerContainerName)

	joined, err := sshService.WaitForWireportServerJoined(3 * time.Minute)
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if joined {
		fmt.Fprintf(stdOut, "   Network: ✅ Joined successfully\n")
		fmt.Fprintf(stdOut, "   🎉 Server has been successfully installed and connected to the wireport network!\n\n")
		fmt.Fprintf(stdOut, "✨ Server Bootstrapping completed successfully!\n")
		return
	}

	fmt.Fprintf(stdOut, "   Network: ❌ Not joined yet\n\n")
	fmt.Fprintf(stdOut, "   The server container is running but could not reach the gateway to complete setup.\n")
	fmt.Fprintf(stdOut, "   Common causes: gateway not reachable, or firewall rules on the server (outbound) or gateway (inbound).\n\n")
	fmt.Fprintf(stdOut, "%s", joinrequests.ConnectivityRequirementsText())
	fmt.Fprintf(stdOut, "   More detail: %s\n\n", config.Config.DocumentationURL)
	fmt.Fprintf(stdOut, "   After opening ports, the server will retry joining automatically — no container restart needed.\n")
	fmt.Fprintf(stdOut, "   Monitor progress: docker logs -f %s\n\n", config.Config.WireportServerContainerName)
	fmt.Fprintf(errOut, "Server bootstrap incomplete: container is running but the server has not joined the wireport network yet\n")
}

func (s *LocalCommandsService) ServerDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	currentNode, err := s.NodesRepository.GetCurrentNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting current node: %v\n", err)
		return
	}

	switch currentNode.Role {
	case types.NodeRoleServer:
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
	fmt.Fprintf(stdOut, "===========================\n\n")

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
	fmt.Fprintf(stdOut, "🛑 Server node Teardown\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, err = sshService.TeardownWireportServer()
	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Teardown Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Teardown Completed\n\n")

	fmt.Fprintf(stdOut, "✨ Server Teardown completed successfully!\n")
}

func (s *LocalCommandsService) ServerList(requestFromNodeID *string, stdOut io.Writer, errOut io.Writer) {
	serverNodes, err := s.NodesRepository.GetNodesByRole(types.NodeRoleServer)

	if err != nil {
		fmt.Fprintf(errOut, "Error getting nodes: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "SERVER PRIVATE IP       LABELS\n")
	fmt.Fprintf(stdOut, "%s\n", strings.Repeat("=", 80))

	if len(serverNodes) > 0 {
		for _, serverNode := range serverNodes {
			ip := serverNode.WGConfig.Interface.Address.String()
			if requestFromNodeID != nil && serverNode.ID == *requestFromNodeID {
				ip += "*"
			}

			labels := strings.Join(serverNode.Labels, ", ")
			if labels == "" {
				labels = "-"
			}

			fmt.Fprintf(stdOut, "%-24s %s\n", ip, labels)
		}
	} else {
		fmt.Fprintf(stdOut, "No servers are registered on this gateway.\nUse 'wireport server new' command to create a new server node join request.\n")
	}
}

func (s *LocalCommandsService) ServerUpgrade(creds *ssh.Credentials, image string, imageTag string, stdOut io.Writer, _ io.Writer) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🔄 wireport Server Upgrade\n")
	fmt.Fprintf(stdOut, "==========================\n\n")

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

	// Upgrade wireport server
	fmt.Fprintf(stdOut, "🔄 Upgrading wireport server...\n")
	fmt.Fprintf(stdOut, "   Server: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	success, err := sshService.UpgradeWireportServer(image, imageTag)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if success {
		fmt.Fprintf(stdOut, "   Status: ✅ Upgraded Successfully\n")
		fmt.Fprintf(stdOut, "   🎉 wireport server node has been successfully upgraded!\n\n")
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   💡 Upgrade process completed but may not have been successful.\n\n")
	}

	fmt.Fprintf(stdOut, "✨ Server Upgrade completed!\n")
}

func (s *LocalCommandsService) NodeLabelAdd(nodeIP string, label string, stdOut io.Writer, errOut io.Writer) {
	node, err := s.NodesRepository.GetServerByWGPrivateIP(nodeIP)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Fprintf(errOut, "No server node found with IP %q\n", nodeIP)
			return
		}
		fmt.Fprintf(errOut, "failed to resolve server node: %v\n", err)
		return
	}

	if err := s.NodesRepository.AddLabelToNode(node.ID, label); err != nil {
		fmt.Fprintf(errOut, "failed to add label: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Added label %q to server node %q\n", label, types.IPToString(node.WGConfig.Interface.Address.IP))
}

func (s *LocalCommandsService) NodeLabelRemove(nodeIP string, label string, stdOut io.Writer, errOut io.Writer) {
	node, err := s.NodesRepository.GetServerByWGPrivateIP(nodeIP)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Fprintf(errOut, "No server node found with IP %q\n", nodeIP)
			return
		}
		fmt.Fprintf(errOut, "failed to resolve server node: %v\n", err)
		return
	}

	if err := s.NodesRepository.RemoveLabelFromNode(node.ID, label); err != nil {
		fmt.Fprintf(errOut, "failed to remove label: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Removed label %q from server node %q\n", label, types.IPToString(node.WGConfig.Interface.Address.IP))
}
