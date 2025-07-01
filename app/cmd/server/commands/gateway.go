package commands

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"wireport/internal/routes"
	"wireport/internal/ssh"
	"wireport/internal/utils"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var GatewayStartConfigureOnly = false

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
			parsedPort, err := strconv.ParseUint(portStr, 10, 32)

			if err != nil {
				return "", "", 0, fmt.Errorf("invalid port number: %s", portStr)
			}

			if parsedPort > 65535 {
				return "", "", 0, fmt.Errorf("port number must be between 0 and 65535")
			}

			port = uint(parsedPort)
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
func buildSSHCredentials(cmd *cobra.Command, args []string, useGatewayNodeIfNoArgs bool) (*ssh.Credentials, error) {
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
		if !useGatewayNodeIfNoArgs {
			return nil, fmt.Errorf("SSH host is required. Use positional argument (username@hostname[:port])")
		}

		// If no positional argument, try to get from database
		gatewaytNode, err := nodesRepository.GetGatewayNode()
		if err != nil {
			return nil, fmt.Errorf("failed to get gateway node from database: %v", err)
		}
		if gatewaytNode == nil {
			return nil, fmt.Errorf("no gateway node found in database")
		}
		if gatewaytNode.WGPublicIP == nil {
			return nil, fmt.Errorf("gateway node public IP not found in database")
		}

		// Use the public IP as the host
		creds.Host = *gatewaytNode.WGPublicIP
		creds.Port = 22 // Default SSH port

		fmt.Printf("🔍 Using the bootstrapped gateway node: %s\n", creds.Host)
		fmt.Printf("👤 Enter SSH username: ")
		_, err = fmt.Scanln(&creds.Username)

		if err != nil {
			return nil, fmt.Errorf("failed to read SSH username: %v", err)
		}

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

var GatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "wireport gateway commands",
	Long:  `Manage wireport gateway node: configure the gateway node and start the wireport gateway node`,
}

var StartGatewayCmd = &cobra.Command{
	Use:   "start",
	Short: "Start wireport in gateway mode",
	Long:  `Start wireport in gateway mode. It will handle network connections and state management.`,
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayPublicIP, err := utils.GetPublicIP()

		if err != nil {
			cmd.PrintErrf("Error: %v\n", err)
			return
		}

		router := routes.Router(dbInstance)

		commandsService.GatewayStart(*gatewayPublicIP, nodesRepository, publicServicesRepository, cmd.OutOrStdout(), cmd.ErrOrStderr(), GatewayStartConfigureOnly, router)
	},
}

var StatusGatewayCmd = &cobra.Command{
	Use:   "status [username@hostname[:port]]",
	Short: "Check wireport gateway node status",
	Long: `Check the status of wireport gateway node: SSH connection, Docker installation, and wireport status.

If no username@hostname[:port] is provided, the command will use the bootstrapped gateway node.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Build credentials from positional argument or flags
		creds, err := buildSSHCredentials(cmd, args, true)

		if err != nil {
			cmd.PrintErrf("Error: %v\n", err)
			return
		}

		commandsService.GatewayStatus(creds, cmd.OutOrStdout())
	},
}

var UpGatewayCmd = &cobra.Command{
	Use:   "up username@hostname[:port]",
	Short: "Start wireport gateway node",
	Long:  `Start wireport gateway node. It will install wireport on the gateway node.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, true)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayUp(creds, cmd.OutOrStdout(), cmd.ErrOrStderr(), nodesRepository)
	},
}

var DownGatewayCmd = &cobra.Command{
	Use:   "down [username@hostname[:port]]",
	Short: "Stop wireport gateway node",
	Long:  `Stop wireport gateway node. It will stop the wireport gateway node and remove all data from the gateway node.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, true)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayDown(creds, cmd.OutOrStdout(), cmd.ErrOrStderr(), nodesRepository)
	},
}

var UpgradeGatewayCmd = &cobra.Command{
	Use:   "upgrade [username@hostname[:port]]",
	Short: "Upgrade wireport gateway node",
	Long:  `Upgrade wireport gateway node. It will upgrade the wireport gateway node to the latest version.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, true)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayUpgrade(creds, cmd.OutOrStdout(), cmd.ErrOrStderr(), nodesRepository)
	},
}

func init() {
	GatewayCmd.AddCommand(StartGatewayCmd)
	GatewayCmd.AddCommand(StatusGatewayCmd)
	GatewayCmd.AddCommand(UpGatewayCmd)
	GatewayCmd.AddCommand(DownGatewayCmd)
	GatewayCmd.AddCommand(UpgradeGatewayCmd)

	StartGatewayCmd.Flags().BoolVar(&GatewayStartConfigureOnly, "configure", false, "Configure wireport in gateway mode without making it available for external connections")

	StatusGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")

	UpGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	DownGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")

	UpgradeGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
}
