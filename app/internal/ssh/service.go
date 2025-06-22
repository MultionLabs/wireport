package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/templates"
	"wireport/version"

	"github.com/aymerick/raymond"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// Service provides SSH functionality for wireport bootstrapping
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
		// Use private key file
		keyBytes, err := os.ReadFile(creds.PrivateKeyPath)
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

	stdout, err := s.client.Run(command)

	result := &CommandResult{
		Stdout: strings.TrimSpace(string(stdout)),
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
	checkContainerCmd := fmt.Sprintf("docker ps -a --filter name=^/%s$ --format '{{.Status}}'", config.Config.WireportHostContainerName)
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

func (s *Service) InstallWireport() (bool, error) {
	isRunning, err := s.IsWireportHostContainerRunning()

	if err != nil {
		return false, err
	}

	if isRunning {
		return true, nil
	}

	installCmdTemplate, err := templates.Scripts.ReadFile(config.Config.BootstrapHostScriptTemplatePath)

	if err != nil {
		return false, err
	}

	tpl, err := raymond.Parse(string(installCmdTemplate))

	if err != nil {
		return false, err
	}

	installCmdStr, err := tpl.Exec(map[string]string{
		"wireportHostContainerName":  config.Config.WireportHostContainerName,
		"wireportHostContainerImage": config.Config.WireportHostContainerImage,
		"wireportVersion":            version.Version,
	})

	if err != nil {
		return false, err
	}

	cmdResult, err := s.executeCommand(installCmdStr)

	if err != nil {
		return false, err
	}

	if cmdResult.ExitCode != 0 {
		return false, nil
	}

	return true, nil
}

func (s *Service) IsWireportHostContainerRunning() (bool, error) {
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

	checkContainerCmd := fmt.Sprintf("docker ps -a --filter name=^/%s$ --format '{{.Names}}'", config.Config.WireportHostContainerName)
	result, err := s.executeCommand(checkContainerCmd)

	if err != nil {
		return false, err
	}

	if result.ExitCode != 0 {
		return false, nil
	}

	if result.ExitCode == 0 && strings.TrimSpace(result.Stdout) == "" {
		return false, nil
	}

	return true, nil
}
