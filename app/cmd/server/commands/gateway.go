package commands

import (
	"fmt"
	"wireport/internal/routes"
	"wireport/internal/ssh"
	"wireport/internal/utils"
	"wireport/version"

	"github.com/spf13/cobra"
)

var GatewayStartConfigureOnly = false
var GatewaySSHKeyPassEmpty = false
var GatewayDockerImageTag = version.Version

var GatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "wireport gateway commands",
	Long:  `Manage wireport gateway node lifecycle and configuration.`,
}

var StartGatewayCmd = &cobra.Command{
	Use:   "start",
	Short: "Start wireport in gateway mode",
	Long:  `Start wireport in gateway mode. wireport in gateway mode will handle WireGuard network, server, client and service management. This command is only relevant for gateway nodes.`,
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayPublicIP, err := utils.GetPublicIP()

		if err != nil {
			cmd.PrintErrf("Error: %v\n", err)
			return
		}

		router := routes.Router(dbInstance)

		commandsService.GatewayStart(*gatewayPublicIP, cmd.OutOrStdout(), cmd.ErrOrStderr(), GatewayStartConfigureOnly, router)
	},
}

var StatusGatewayCmd = &cobra.Command{
	Use:   "status [username@hostname[:port]]",
	Short: "Check wireport gateway node status",
	Long: `Check the status of a wireport gateway node: SSH connection, Docker installation, and wireport status.

If no username@hostname[:port] is provided, the command will use the IP address of the bootstrapped gateway node and will prompt user for the SSH credentials.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Build credentials from positional argument or flags
		creds, err := buildSSHCredentials(cmd, args, true, false, GatewaySSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayStatus(creds, cmd.OutOrStdout())
	},
}

var UpGatewayCmd = &cobra.Command{
	Use:   "up username@hostname[:port]",
	Short: "Bootstrap wireport gateway node",
	Long:  `Bootstrap wireport gateway node: install and configure wireport software in gateway mode on it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, true, true, GatewaySSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayUp(creds, GatewayDockerImageTag, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var DownGatewayCmd = &cobra.Command{
	Use:   "down [username@hostname[:port]]",
	Short: "Teardown wireport gateway node",
	Long:  `Teardown wireport gateway node: stop the wireport gateway software and remove all the data and configuration from the gateway node.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var creds *ssh.Credentials
		var err error

		cmd.Printf("🔴 WARNING: This command will destroy all wireport data and configuration on the gateway node.\nAre you sure you want to continue? (y/n): ")

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

		if len(args) > 0 {
			creds, err = buildSSHCredentials(cmd, args, true, false, GatewaySSHKeyPassEmpty)

			if err != nil {
				cmd.PrintErrf("❌ Error: %v\n", err)
				return
			}
		}

		commandsService.GatewayDown(creds, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var UpgradeGatewayCmd = &cobra.Command{
	Use:   "upgrade [username@hostname[:port]]",
	Short: "Upgrade wireport gateway node",
	Long:  `Upgrade wireport gateway node to the latest version of the wireport gateway docker image. This command is only relevant for bootstrapped gateway nodes.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, true, false, GatewaySSHKeyPassEmpty)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.GatewayUpgrade(creds, GatewayDockerImageTag, cmd.OutOrStdout(), cmd.ErrOrStderr())
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
	StatusGatewayCmd.Flags().BoolVar(&GatewaySSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")

	UpGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	UpGatewayCmd.Flags().BoolVar(&GatewaySSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")
	UpGatewayCmd.Flags().StringVar(&GatewayDockerImageTag, "image-tag", version.Version, "Image tag to use for the wireport gateway container")

	DownGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	DownGatewayCmd.Flags().BoolVar(&GatewaySSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")

	UpgradeGatewayCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	UpgradeGatewayCmd.Flags().BoolVar(&GatewaySSHKeyPassEmpty, "ssh-key-pass-empty", false, "Skip SSH key passphrase prompt (for passwordless SSH keys)")
	UpgradeGatewayCmd.Flags().StringVar(&GatewayDockerImageTag, "image-tag", version.Version, "Image tag to use for the wireport gateway container")
}
