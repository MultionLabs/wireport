package commands

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"wireport/cmd/server/config"
	"wireport/internal/nodes/types"
	"wireport/internal/ssh"
)

func (s *LocalCommandsService) GatewayStart(gatewayPublicIP string, stdOut io.Writer, errOut io.Writer, gatewayStartConfigureOnly bool, router http.Handler) {
	gatewayNode, err := s.NodesRepository.EnsureGatewayNode(types.IPMarshable{
		IP: net.ParseIP(gatewayPublicIP),
	}, config.Config.WGPublicPort, gatewayPublicIP, config.Config.ControlServerPort)

	if err != nil {
		fmt.Fprintf(errOut, "wireport gateway node start failed: %v\n", err)
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
				serverError <- fmt.Errorf("Failed to get TLS config for the gateway control server: %v", err)
				return
			}

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

	publicServices, err := s.PublicServicesRepository.GetAll()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
		return
	}

	err = gatewayNode.SaveConfigs(publicServices, true)

	if err != nil {
		fmt.Fprintf(errOut, "Failed to save configs: %v\n", err)
		return
	}

	if !gatewayStartConfigureOnly {
		fmt.Fprintf(stdOut, "wireport server has started with mTLS on gateway: %s\n", gatewayPublicIP)
	} else {
		fmt.Fprintf(stdOut, "wireport has been configured on the gateway: %s\n", gatewayPublicIP)
	}

	if !gatewayStartConfigureOnly {
		// Block on the server error channel
		if err := <-serverError; err != nil {
			fmt.Fprintf(errOut, "Server error: %v\n", err)
		}
	}
}

func (s *LocalCommandsService) GatewayStatus(creds *ssh.Credentials, stdOut io.Writer) {
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
	fmt.Fprintf(stdOut, "🚀 wireport Gateway Status\n")
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

		fmt.Fprintf(stdOut, "   💡 Run 'wireport gateway up %s@%s:%d' to bootstrap wireport gateway and start it.\n", creds.Username, creds.Host, creds.Port)
	}
	fmt.Fprintf(stdOut, "\n")

	fmt.Fprintf(stdOut, "✨ Gateway Status check completed successfully!\n")
}

func (s *LocalCommandsService) GatewayUp(creds *ssh.Credentials, image string, imageTag string, stdOut io.Writer, errOut io.Writer) {
	sshService := ssh.NewService()

	fmt.Fprintf(errOut, "🚀 wireport Gateway Bootstrapping\n")
	fmt.Fprintf(errOut, "=================================\n\n")

	// SSH Connection
	fmt.Fprintf(errOut, "📡 Connecting to gateway...\n")
	fmt.Fprintf(errOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	err := sshService.Connect(creds)

	if err != nil {
		fmt.Fprintf(errOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(errOut, "   Error:  %v\n\n", err)
		return
	}

	defer sshService.Close()

	fmt.Fprintf(errOut, "   Status: ✅ Connected\n\n")
	fmt.Fprintf(errOut, "🔍 Checking current status...\n")

	isRunning, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(errOut, "   Status: ❌ Check Failed\n")
		fmt.Fprintf(errOut, "   Error:  %v\n\n", err)
		return
	}

	if isRunning {
		fmt.Fprintf(errOut, "   Status: ✅ Already Running\n")
		fmt.Fprintf(errOut, "   💡 wireport gateway container is already running on this gateway and bootstrapping is not required.\n\n")
		return
	}

	fmt.Fprintf(errOut, "   Status: ❌ Not Running\n")
	fmt.Fprintf(errOut, "   💡 Proceeding with installation...\n\n")
	fmt.Fprintf(errOut, "📦 Installing wireport gateway...\n")
	fmt.Fprintf(errOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	_, clientJoinToken, err := sshService.InstallWireportGateway(image, imageTag)

	if err != nil {
		fmt.Fprintf(errOut, "   Status: ❌ Installation Failed\n")
		fmt.Fprintf(errOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(errOut, "   Status: ✅ Installation Completed\n\n")
	fmt.Fprintf(errOut, "✅ Verifying installation...\n")

	installationConfirmed, err := sshService.IsWireportGatewayContainerRunning()
	if err != nil {
		fmt.Fprintf(errOut, "   Status: ❌ Verification Failed\n")
		fmt.Fprintf(errOut, "   Error:  %v\n\n", err)
		return
	}

	if installationConfirmed {
		fmt.Fprintf(errOut, "   Status: ✅ Verified Successfully, Running\n")
		fmt.Fprintf(errOut, "   🎉 wireport has been successfully installed and started on the gateway!\n\n")
	} else {
		fmt.Fprintf(errOut, "   Status: ❌ Verified Failed\n")
		fmt.Fprintf(errOut, "   💡 wireport container was not found running after installation.\n\n")
	}

	if clientJoinToken != nil {
		fmt.Fprintf(errOut, "   🔑 Applying Client Join Token: %s...\n", (*clientJoinToken)[:20])

		s.Join(stdOut, errOut, *clientJoinToken)

		fmt.Fprintf(errOut, "   ✅ Client Join Token Applied\n\n")
	}

	fmt.Fprintf(errOut, "✨ Gateway Bootstrapping completed successfully!\n")
}

func (s *LocalCommandsService) GatewayDown(creds *ssh.Credentials, stdOut io.Writer, errOut io.Writer) {
	// First, try to determine if we are executing on a gateway node or locally
	currentNode, err := s.NodesRepository.GetCurrentNode()

	if err == nil && currentNode != nil && currentNode.Role == types.NodeRoleGateway {

		return
	}

	// Remote execution – credentials are required
	if creds == nil {
		fmt.Fprintf(errOut, "Error: SSH credentials are required\n")
		return
	}

	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🚀 wireport Gateway Teardown\n")
	fmt.Fprintf(stdOut, "============================\n\n")

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
		fmt.Fprintf(stdOut, "   💡 wireport gateway is not running, teardown is not required\n\n")
		return
	}

	// Teardown wireport gateway
	fmt.Fprintf(stdOut, "🛑 Gateway node Teardown\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	success, err := sshService.TeardownWireportGateway()

	if err != nil || !success {
		fmt.Fprintf(stdOut, "   Status: ❌ Teardown Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Teardown Completed\n\n")

	// reset local state
	fmt.Fprintf(stdOut, "🔄 Resetting local state...\n")
	err = s.NodesRepository.DeleteAll()

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	fmt.Fprintf(stdOut, "   Status: ✅ Completed\n\n")

	fmt.Fprintf(stdOut, "✨ Gateway Teardown completed successfully!\n")
}

func (s *LocalCommandsService) GatewayUpgrade(creds *ssh.Credentials, image string, imageTag string, stdOut io.Writer, _ io.Writer) {
	sshService := ssh.NewService()

	fmt.Fprintf(stdOut, "🔄 wireport Gateway Upgrade\n")
	fmt.Fprintf(stdOut, "===========================\n\n")

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

	// Upgrade wireport gateway
	fmt.Fprintf(stdOut, "🔄 Upgrading wireport gateway...\n")
	fmt.Fprintf(stdOut, "   Gateway: %s@%s:%d\n", creds.Username, creds.Host, creds.Port)

	success, err := sshService.UpgradeWireportGateway(image, imageTag)

	if err != nil {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   Error:  %v\n\n", err)
		return
	}

	if success {
		fmt.Fprintf(stdOut, "   Status: ✅ Upgraded Successfully\n")
		fmt.Fprintf(stdOut, "   🎉 wireport gateway node has been successfully upgraded!\n\n")
	} else {
		fmt.Fprintf(stdOut, "   Status: ❌ Failed\n")
		fmt.Fprintf(stdOut, "   💡 Upgrade process completed but may not have been successful.\n\n")
	}

	fmt.Fprintf(stdOut, "✨ Gateway Upgrade completed!\n")
}
