package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/templates"

	"github.com/aymerick/raymond"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// Service provides SSH functionality for wireport gateway and server nodes
type Service struct {
	client *goph.Client
	creds  *Credentials
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Connect(creds *Credentials) error {
	var authMethods []ssh.AuthMethod
	var err error

	// Determine chosen authentication method
	if creds.PrivateKeyPath != "" {
		var keyBytes []byte
		keyBytes, err = os.ReadFile(creds.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrFailedToCreateAuth, err)
		}

		var signer ssh.Signer
		if creds.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(creds.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyBytes)
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrFailedToCreateAuth, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if len(creds.PrivateKeyData) > 0 {
		// Use private key data
		var signer ssh.Signer
		if creds.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(creds.PrivateKeyData, []byte(creds.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(creds.PrivateKeyData)
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrFailedToCreateAuth, err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if creds.Password != "" {
		// Use password authentication
		authMethods = append(authMethods, ssh.Password(creds.Password))
	} else {
		return ErrNoAuthMethodProvided
	}

	// Create SSH client with auto-accept host keys
	sshConfig := &ssh.ClientConfig{
		User:            creds.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	hostPort := net.JoinHostPort(creds.Host, fmt.Sprintf("%d", creds.Port))

	conn, err := net.DialTimeout("tcp", hostPort, sshConfig.Timeout)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrFailedToCreateSSHClient, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, hostPort, sshConfig)

	if err != nil {
		conn.Close()
		return fmt.Errorf("%w: %v", ErrFailedToCreateSSHClient, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)

	session, err := client.NewSession()

	if err != nil {
		client.Close()
		return fmt.Errorf("%w: %v", ErrFailedToTestSSHConnection, err)
	}

	defer session.Close()

	err = session.Run("echo 'connection test'")

	if err != nil {
		client.Close()
		return fmt.Errorf("%w: %v", ErrFailedToTestSSHConnection, err)
	}

	s.client = &goph.Client{Client: client}
	s.creds = creds
	return nil
}

func (s *Service) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *Service) executeCommand(command string) (*CommandResult, error) {
	if s.client == nil {
		return nil, ErrSSHConnectionNotEstablished
	}

	// Use Command method to get separate stdout and stderr
	cmd, err := s.client.Command(command)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	result := &CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}

	if err != nil {
		result.Error = err
		if exitErr, ok := err.(*ssh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			result.ExitCode = -1
		}
	} else {
		result.ExitCode = 0
	}

	return result, nil
}

func (s *Service) IsDockerInstalled() (bool, error) {
	result, err := s.executeCommand("which docker")

	if err != nil {
		return false, err
	}

	return result.ExitCode == 0, nil
}

func (s *Service) IsDockerAccessible() (bool, error) {
	result, err := s.executeCommand("docker ps")
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

func (s *Service) GetDockerVersion() (string, error) {
	result, err := s.executeCommand("docker --version")
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", ErrFailedToGetDockerVersion
	}
	return result.Stdout, nil
}

func (s *Service) GetWireportContainerStatus() (string, error) {
	checkContainerCmd := fmt.Sprintf("docker ps -a --filter name=^/%s$ --format '{{.Status}}'", config.Config.WireportGatewayContainerName)
	result, err := s.executeCommand(checkContainerCmd)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", ErrFailedToGetContainerStatus
	}
	return result.Stdout, nil
}

func (s *Service) GetWireportNetworkStatus() (string, error) {
	checkNetworkCmd := fmt.Sprintf("docker network ls --filter name=^%s$ --format '{{.Name}}'", config.Config.DockerNetworkName)
	result, err := s.executeCommand(checkNetworkCmd)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", ErrFailedToGetNetworkStatus
	}
	return result.Stdout, nil
}

func (s *Service) InstallWireportGateway(imageTag string) (bool, *string, error) {
	isRunning, err := s.IsWireportGatewayContainerRunning()

	if err != nil {
		return false, nil, err
	}

	if isRunning {
		fmt.Println("wireport gateway container is already running, skipping installation")
		return true, nil, nil
	}

	installCmdTemplate, err := templates.Scripts.ReadFile(config.Config.UpGatewayScriptTemplatePath)

	if err != nil {
		return false, nil, err
	}

	tpl, err := raymond.Parse(string(installCmdTemplate))

	if err != nil {
		return false, nil, err
	}

	// 1. install and start wireport

	installCmdStr, err := tpl.Exec(map[string]string{
		"wireportGatewayContainerName":  config.Config.WireportGatewayContainerName,
		"wireportGatewayContainerImage": fmt.Sprintf("%s:%s", config.Config.WireportGatewayContainerImage, imageTag),
	})

	if err != nil {
		return false, nil, err
	}

	cmdResult, err := s.executeCommand(installCmdStr)

	if err != nil {
		return false, nil, err
	}

	if cmdResult.ExitCode != 0 {
		return false, nil, fmt.Errorf("failed to install wireport: %s", cmdResult.Stderr)
	}

	// 2. generate new client

	clientJoinToken, err := s.createClientJoinToken()

	if err != nil {
		return false, nil, err
	}

	return true, clientJoinToken, nil
}

func (s *Service) createClientJoinToken() (*string, error) {
	createClientCmdTemplate, err := templates.Scripts.ReadFile(config.Config.NewClientScriptTemplatePath)

	if err != nil {
		return nil, err
	}

	tpl, err := raymond.Parse(string(createClientCmdTemplate))

	if err != nil {
		return nil, err
	}

	createClientCmdStr, err := tpl.Exec(map[string]string{
		"wireportGatewayContainerName": config.Config.WireportGatewayContainerName,
	})

	if err != nil {
		return nil, err
	}

	cmdResult, err := s.executeCommand(createClientCmdStr)

	if err != nil {
		return nil, err
	}

	if cmdResult.ExitCode != 0 {
		return nil, fmt.Errorf("failed to create client on the gateway: %s", cmdResult.Stderr)
	}

	clientJoinToken := strings.TrimSpace(cmdResult.Stdout)

	return &clientJoinToken, nil
}

func (s *Service) IsWireportGatewayContainerRunning() (bool, error) {
	dockerInstalled, err := s.IsDockerInstalled()

	if err != nil {
		return false, err
	}

	if !dockerInstalled {
		return false, nil
	}

	dockerAccessible, err := s.IsDockerAccessible()

	if err != nil {
		return false, err
	}

	if !dockerAccessible {
		return false, nil
	}

	checkContainerCmd := fmt.Sprintf("docker ps --filter name=^/%s$ --format '{{.Names}}'", config.Config.WireportGatewayContainerName)
	result, err := s.executeCommand(checkContainerCmd)

	if err != nil {
		return false, err
	}

	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) == "" {
		return false, nil
	}

	return true, nil
}

func (s *Service) IsWireportServerContainerRunning() (bool, error) {
	dockerInstalled, err := s.IsDockerInstalled()

	if err != nil {
		return false, err
	}

	if !dockerInstalled {
		return false, nil
	}

	dockerAccessible, err := s.IsDockerAccessible()

	if err != nil {
		return false, err
	}

	if !dockerAccessible {
		return false, nil
	}

	checkContainerCmd := fmt.Sprintf("docker ps --filter name=^/%s$ --format '{{.Names}}'", config.Config.WireportServerContainerName)
	result, err := s.executeCommand(checkContainerCmd)

	if err != nil {
		return false, err
	}

	if result.ExitCode != 0 || strings.TrimSpace(result.Stdout) == "" {
		return false, nil
	}

	return true, nil
}

func (s *Service) GetWireportServerContainerStatus() (string, error) {
	checkContainerCmd := fmt.Sprintf("docker ps -a --filter name=^/%s$ --format '{{.Status}}'", config.Config.WireportServerContainerName)
	result, err := s.executeCommand(checkContainerCmd)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", ErrFailedToGetContainerStatus
	}
	return result.Stdout, nil
}

func (s *Service) InstallWireportServer(serverJoinToken string, imageTag string) (bool, error) {
	isRunning, err := s.IsWireportServerContainerRunning()

	if err != nil {
		return false, err
	}

	if isRunning {
		fmt.Println("wireport server container is already running, skipping installation")
		return true, nil
	}

	// Install wireport server using the connect script
	installCmdTemplate, err := templates.Scripts.ReadFile(config.Config.UpServerScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(installCmdTemplate))

	if err != nil {
		return false, err
	}

	installCmdStr, err := tpl.Exec(map[string]string{
		"wireportServerContainerName":  config.Config.WireportServerContainerName,
		"wireportServerContainerImage": fmt.Sprintf("%s:%s", config.Config.WireportServerContainerImage, imageTag),
		"serverJoinToken":              serverJoinToken,
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(installCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, fmt.Errorf("failed to install wireport server: %s", cmdResult.Stderr)
	}

	return true, nil
}

func (s *Service) TeardownWireportServer() (bool, error) {
	stopCmdTemplate, err := templates.Scripts.ReadFile(config.Config.DownServerScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(stopCmdTemplate))

	if err != nil {
		return false, err
	}

	stopCmdStr, err := tpl.Exec(map[string]string{
		"wireportServerContainerName":  config.Config.WireportServerContainerName,
		"wireportServerContainerImage": config.Config.WireportServerContainerImage,
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(stopCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, fmt.Errorf("failed to stop wireport server: %s", cmdResult.Stderr)
	}

	return true, nil
}

func (s *Service) UpgradeWireportGateway(imageTag string) (bool, error) {
	upgradeCmdTemplate, err := templates.Scripts.ReadFile(config.Config.UpgradeGatewayScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(upgradeCmdTemplate))

	if err != nil {
		return false, err
	}

	upgradeCmdStr, err := tpl.Exec(map[string]string{
		"wireportGatewayContainerName":  config.Config.WireportGatewayContainerName,
		"wireportGatewayContainerImage": fmt.Sprintf("%s:%s", config.Config.WireportGatewayContainerImage, imageTag),
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(upgradeCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, fmt.Errorf("failed to upgrade wireport gateway: %s", cmdResult.Stderr)
	}

	return true, nil
}

func (s *Service) UpgradeWireportServer(imageTag string) (bool, error) {
	upgradeCmdTemplate, err := templates.Scripts.ReadFile(config.Config.UpgradeServerScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(upgradeCmdTemplate))

	if err != nil {
		return false, err
	}

	upgradeCmdStr, err := tpl.Exec(map[string]string{
		"wireportServerContainerName":  config.Config.WireportServerContainerName,
		"wireportServerContainerImage": fmt.Sprintf("%s:%s", config.Config.WireportServerContainerImage, imageTag),
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(upgradeCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, fmt.Errorf("failed to upgrade wireport server: %s", cmdResult.Stderr)
	}

	return true, nil
}

func (s *Service) TeardownWireportGateway() (bool, error) {
	teardownCmdTemplate, err := templates.Scripts.ReadFile(config.Config.DownGatewayScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(teardownCmdTemplate))

	if err != nil {
		return false, err
	}

	teardownCmdStr, err := tpl.Exec(map[string]string{
		"wireportGatewayContainerName":  config.Config.WireportGatewayContainerName,
		"wireportGatewayContainerImage": config.Config.WireportGatewayContainerImage,
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(teardownCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, fmt.Errorf("failed to teardown wireport gateway: %s", cmdResult.Stderr)
	}

	return true, nil
}
