package commands

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"wireport/internal/ssh"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var HostStartConfigureOnly bool = false

func readPasswordSecurely(prompt string) (string, error) {
	// readPasswordSecurely reads a password from the terminal without echoing
	fmt.Print(prompt)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Add newline after password input
	if err != nil {
		return "", err
	}
	return string(bytePassword), nil
}

// parseSSHURL parses an SSH URL in the format username@hostname:port or username@hostname
// Returns username, hostname, port, and any error
func parseSSHURL(sshURL string) (username, hostname string, port uint, err error) {
	// Default port
	port = 22

	// Check if URL contains port
	if strings.Contains(sshURL, ":") {
		parts := strings.Split(sshURL, ":")
		if len(parts) != 2 {
			return "", "", 0, fmt.Errorf("invalid SSH URL format: %s", sshURL)
		}

		// Parse port
		if portStr := parts[1]; portStr != "" {
			if parsedPort, err := strconv.ParseUint(portStr, 10, 32); err != nil {
				return "", "", 0, fmt.Errorf("invalid port number: %s", portStr)
			} else {
				port = uint(parsedPort)
			}
		}

		sshURL = parts[0]
	}

	// Parse username@hostname
	if strings.Contains(sshURL, "@") {
		parts := strings.Split(sshURL, "@")
		if len(parts) != 2 {
			return "", "", 0, fmt.Errorf("invalid SSH URL format: %s", sshURL)
		}
		username = parts[0]
		hostname = parts[1]
	} else {
		return "", "", 0, fmt.Errorf("username is required in SSH URL format: username@hostname[:port]")
	}

	if username == "" {
		return "", "", 0, fmt.Errorf("username cannot be empty")
	}
	if hostname == "" {
		return "", "", 0, fmt.Errorf("hostname cannot be empty")
	}

	return username, hostname, port, nil
}

// buildSSHCredentials builds SSH credentials from positional arguments or database
func buildSSHCredentials(cmd *cobra.Command, args []string) (*ssh.Credentials, error) {
	creds := &ssh.Credentials{}

	// Try to parse SSH URL from positional argument first
	if len(args) > 0 {
		username, hostname, port, err := parseSSHURL(args[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH URL '%s': %v", args[0], err)
		}
		creds.Username = username
		creds.Host = hostname
		creds.Port = port
	} else {
		// If no positional argument, try to get from database
		hostNode, err := nodes_repository.GetHostNode()
		if err != nil {
			return nil, fmt.Errorf("failed to get host node from database: %v", err)
		}
		if hostNode == nil {
			return nil, fmt.Errorf("no host node found in database")
		}
		if hostNode.WGPublicIp == nil {
			return nil, fmt.Errorf("host node public IP not found in database")
		}

		// Use the public IP as the host
		creds.Host = *hostNode.WGPublicIp
		creds.Port = 22 // Default SSH port

		fmt.Printf("🔍 Using the bootstrapped host node: %s\n", creds.Host)
		fmt.Printf("👤 Enter SSH username: ")
		fmt.Scanln(&creds.Username)

		if creds.Username == "" {
			return nil, fmt.Errorf("SSH username is required")
		}
	}

	if creds.Host == "" {
		return nil, fmt.Errorf("SSH host is required. Use positional argument (username@hostname[:port])")
	}
	if creds.Username == "" {
		return nil, fmt.Errorf("SSH username is required. Use positional argument (username@hostname[:port])")
	}

	// Handle authentication method
	if keyPath := cmd.Flag("ssh-key-path").Value.String(); keyPath != "" {
		creds.PrivateKeyPath = keyPath
		if passphrase, err := readPasswordSecurely("🔒 Enter SSH key passphrase (leave empty if none): "); err == nil && passphrase != "" {
			creds.Passphrase = passphrase
		}
	} else {
		if password, err := readPasswordSecurely("🔒 Enter SSH password: "); err == nil {
			creds.Password = password
		} else {
			return nil, fmt.Errorf("failed to read password: %v", err)
		}
	}

	if creds.PrivateKeyPath == "" && creds.Password == "" {
		return nil, fmt.Errorf("SSH authentication is required. Use --ssh-key-path flag or provide password interactively")
	}

	return creds, nil
}

var HostCmd = &cobra.Command{
	Use:   "host",
	Short: "wireport host commands",
	Long:  `Manage wireport host node: configure the host node and start the wireport host node`,
}

var StartHostCmd = &cobra.Command{
	Use:   "start",
	Short: "Start wireport in host mode",
	Long:  `Start wireport in host mode. It will handle network connections and state management.`,
	Run: func(cmd *cobra.Command, args []string) {
		commandsService.HostStart(join_requests_service, nodes_repository, public_services_repository, dbInstance, cmd.OutOrStdout(), cmd.ErrOrStderr(), HostStartConfigureOnly)
	},
}

var StatusHostCmd = &cobra.Command{
	Use:   "status [username@hostname[:port]]",
	Short: "Check wireport host node status",
	Long: `Check the status of wireport host node: SSH connection, Docker installation, and wireport status.

If no username@hostname[:port] is provided, the command will use the bootstrapped host node.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Build credentials from positional argument or flags
		creds, err := buildSSHCredentials(cmd, args)

		if err != nil {
			cmd.PrintErrf("Error: %v\n", err)
			return
		}

		commandsService.HostStatus(creds, cmd.OutOrStdout())
	},
}

var BootstrapHostCmd = &cobra.Command{
	Use:   "bootstrap username@hostname[:port]",
	Short: "Bootstrap wireport host node",
	Long:  `Bootstrap wireport host node. It will install wireport on the host node.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.HostBootstrap(creds, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	HostCmd.AddCommand(StartHostCmd)
	HostCmd.AddCommand(StatusHostCmd)
	HostCmd.AddCommand(BootstrapHostCmd)

	StartHostCmd.Flags().BoolVar(&HostStartConfigureOnly, "configure", false, "Configure wireport in host mode without making it available for external connections")

	StatusHostCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")

	BootstrapHostCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
}
