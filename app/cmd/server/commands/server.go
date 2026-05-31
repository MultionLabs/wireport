package commands

import (
	"fmt"
	"strings"
	"wireport/cmd/server/config"
	"wireport/internal/nodes/types"
	"wireport/internal/ssh"
	"wireport/version"

	"github.com/spf13/cobra"
)

var forceServerCreation = false
var quietServerCreation = false
var dockerSubnet = ""
var ServerSSHKeyPassEmpty = false
var ServerDockerImage = config.Config.WireportServerContainerImage
var ServerDockerImageTag = version.Version
var forceServerTeardown = false

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "wireport server commands",
	Long:  `Manage connected wireport server nodes and create join-requests for connecting new servers to the wireport network.`,
}

var NewServerCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a server to wireport network",
	Long:  `Create a new join-request for connecting a server to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command help)`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServerNew(cmd.OutOrStdout(), cmd.ErrOrStderr(), forceServerCreation, quietServerCreation, dockerSubnet)
	},
}

var StartServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start wireport in server mode",
	Long:  `Start wireport in server mode. This command is only relevant for server nodes after they joined the network.`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServerStart(cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var StatusServerCmd = &cobra.Command{
	Use:   "status username@hostname[:port]",
	Short: "Check wireport server node status",
	Long:  `Check the status of a wireport server node: SSH connection, Docker installation, and wireport server status.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Build credentials from positional argument or flags
		creds, err := buildSSHCredentials(cmd, args, false, false, ServerSSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.ServerStatus(creds, cmd.OutOrStdout())
	},
}

var UpServerCmd = &cobra.Command{
	Use:   "up username@hostname[:port]",
	Short: "Bootstrap a wireport server node",
	Long:  `Bootstrap a wireport server node: install and configure wireport software in server mode on it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, false, false, ServerSSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		if dockerSubnet != "" {
			// validate the subnet format
			_, err := types.ParseIPNetMarshable(dockerSubnet, true)

			if err != nil {
				cmd.PrintErrf("❌ Failed to parse Docker subnet: %v\n", err)
				return
			}
		}

		commandsService.ServerUp(creds, ServerDockerImage, ServerDockerImageTag, cmd.OutOrStdout(), cmd.ErrOrStderr(), dockerSubnet)
	},
}

var DownServerCmd = &cobra.Command{
	Use:   "down username@hostname[:port]",
	Short: "Teardown wireport server node",
	Long:  `Teardown wireport server node: stop the wireport server software and remove all the data and configuration from the server node, deregister the server node from the wireport network.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var creds *ssh.Credentials
		var err error

		if !forceServerTeardown {
			cmd.Printf("🔴 WARNING: This command will destroy all wireport data and configuration on the server node.\nAre you sure you want to continue? (y/n): ")

			var confirm string
			_, err = fmt.Scanln(&confirm)

			if err != nil {
				cmd.PrintErrf("❌ Error: %v\n", err)
				return
			}

			if confirm != "y" {
				cmd.PrintErrf("❌ Aborted\n")
				return
			}
		}

		if len(args) > 0 {
			var err error
			creds, err = buildSSHCredentials(cmd, args, false, false, ServerSSHKeyPassEmpty)

			if err != nil {
				cmd.PrintErrf("❌ Error: %v\n", err)
				return
			}
		}

		commandsService.ServerDown(creds, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var ListServerCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	Long:  `List all servers that are connected to the wireport network`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServerList(nil, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var UpgradeServerCmd = &cobra.Command{
	Use:   "upgrade username@hostname[:port]",
	Short: "Upgrade a server",
	Long:  `Upgrade a server. This command will upgrade the wireport server software to the latest version. This command is only relevant for server nodes after they joined the network.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, false, false, ServerSSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.ServerUpgrade(creds, ServerDockerImage, ServerDockerImageTag, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func validateNodeIPAndLabel(nodeIP string, label string) error {
	if strings.TrimSpace(nodeIP) == "" || strings.TrimSpace(label) == "" {
		return fmt.Errorf("node IP and label must be non-empty")
	}

	return nil
}

var ServerLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "Add or remove labels on server nodes",
	Long: `Manage labels stored on server nodes in the gateway database (e.g. feature flags consumed by automation and experimental features).

Every now and then server nodes synchronize their labels with the gateway database to ensure that the labels are up to date.

On a client node, these commands call the gateway control API over mTLS. On the gateway node, they update the local database directly.`,
}

var ServerLabelAddCmd = &cobra.Command{
	Use:   "add NODE_IP LABEL",
	Short: "Append a label to a server node if it is not already present",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nodeIP := args[0]
		label := args[1]

		if err := validateNodeIPAndLabel(nodeIP, label); err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.NodeLabelAdd(cmd.OutOrStdout(), cmd.ErrOrStderr(), nodeIP, label)
	},
}

var ServerLabelRemoveCmd = &cobra.Command{
	Use:   "remove NODE_IP LABEL",
	Short: "Remove a label from a server node",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nodeIP := args[0]
		label := args[1]

		if err := validateNodeIPAndLabel(nodeIP, label); err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.NodeLabelRemove(cmd.OutOrStdout(), cmd.ErrOrStderr(), nodeIP, label)
	},
}

func init() {
	NewServerCmd.Flags().BoolVarP(&forceServerCreation, "force", "f", false, "Force the creation of a new server, bypassing the join request generation")
	NewServerCmd.Flags().StringVar(&dockerSubnet, "docker-subnet", "", "Specify a custom Docker subnet for the server (e.g. 172.20.0.0/16)")
	NewServerCmd.Flags().BoolVarP(&quietServerCreation, "quiet", "q", false, "Quiet mode, don't print any output except for the join request token")

	ServerCmd.AddCommand(NewServerCmd)
	ServerCmd.AddCommand(StartServerCmd)
	ServerCmd.AddCommand(StatusServerCmd)
	ServerCmd.AddCommand(UpServerCmd)
	ServerCmd.AddCommand(DownServerCmd)
	ServerCmd.AddCommand(ListServerCmd)
	ServerCmd.AddCommand(UpgradeServerCmd)
	ServerLabelCmd.AddCommand(ServerLabelAddCmd)
	ServerLabelCmd.AddCommand(ServerLabelRemoveCmd)
	ServerCmd.AddCommand(ServerLabelCmd)

	StatusServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	StatusServerCmd.Flags().BoolVar(&ServerSSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")

	UpServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	UpServerCmd.Flags().BoolVar(&ServerSSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")
	UpServerCmd.Flags().StringVar(&dockerSubnet, "docker-subnet", "", "Specify a custom Docker subnet for the server (e.g. 172.20.0.0/16)")
	UpServerCmd.Flags().StringVar(&ServerDockerImage, "image", config.Config.WireportServerContainerImage, "Docker image to use for the wireport server container")
	UpServerCmd.Flags().StringVar(&ServerDockerImageTag, "image-tag", version.Version, "Image tag to use for the wireport server container")

	DownServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	DownServerCmd.Flags().BoolVar(&ServerSSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")
	DownServerCmd.Flags().BoolVarP(&forceServerTeardown, "force", "f", false, "Force the teardown of the server node, bypassing the confirmation prompt")

	UpgradeServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	UpgradeServerCmd.Flags().BoolVar(&ServerSSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")
	UpgradeServerCmd.Flags().StringVar(&ServerDockerImage, "image", config.Config.WireportServerContainerImage, "Docker image to use for the wireport server container")
	UpgradeServerCmd.Flags().StringVar(&ServerDockerImageTag, "image-tag", version.Version, "Image tag to use for the wireport server container")
}
