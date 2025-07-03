package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"
	"wireport/internal/ssh"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func readPasswordSecurely(prompt string, stdOut io.Writer, errOut io.Writer, promptToErr bool) (string, error) {
	// readPasswordSecurely reads a password from the terminal without echoing
	if promptToErr {
		fmt.Fprintf(errOut, "%s", prompt)
	} else {
		fmt.Fprintf(stdOut, "%s", prompt)
	}

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))

	if promptToErr {
		fmt.Fprintf(errOut, "\n")
	} else {
		fmt.Fprintf(stdOut, "\n")
	}

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
func buildSSHCredentials(cmd *cobra.Command, args []string, useGatewayNodeIfNoArgs bool, promptToErr bool, sshKeyPassSkip bool) (*ssh.Credentials, error) {
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
		errWrongArgs := fmt.Errorf("wrong arguments; use positional argument to specify the SSH credentials (username@hostname[:port])")
		if !useGatewayNodeIfNoArgs {
			return nil, errWrongArgs
		}

		// If no positional argument, try to get from database
		gatewaytNode, err := nodesRepository.GetGatewayNode()
		if err != nil {
			return nil, fmt.Errorf("failed to get gateway node from database: %v: %v", err, errWrongArgs)
		}
		if gatewaytNode == nil {
			return nil, fmt.Errorf("no gateway node found in database: %v", errWrongArgs)
		}
		if gatewaytNode.GatewayPublicIP == "" {
			return nil, fmt.Errorf("gateway node public IP is not set: %v", errWrongArgs)
		}

		// Use the public IP as the host
		creds.Host = gatewaytNode.GatewayPublicIP
		creds.Port = 22 // Default SSH port

		if promptToErr {
			fmt.Fprintf(cmd.ErrOrStderr(), "🔍 Using the bootstrapped gateway node: %s\n", creds.Host)
			fmt.Fprintf(cmd.ErrOrStderr(), "👤 Enter SSH username: ")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "🔍 Using the bootstrapped gateway node: %s\n", creds.Host)
			fmt.Fprintf(cmd.OutOrStdout(), "👤 Enter SSH username: ")
		}

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

		if !sshKeyPassSkip {
			if passphrase, err := readPasswordSecurely("🔒 Enter SSH key passphrase (leave empty if none): ", cmd.OutOrStdout(), cmd.ErrOrStderr(), promptToErr); err == nil && passphrase != "" {
				creds.Passphrase = passphrase
			}
		}
	} else {
		if password, err := readPasswordSecurely("🔒 Enter SSH password: ", cmd.OutOrStdout(), cmd.ErrOrStderr(), promptToErr); err == nil {
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
