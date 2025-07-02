package commands

import (
	"wireport/internal/nodes/types"
	"wireport/internal/ssh"

	"github.com/spf13/cobra"
)

var forceServerCreation = false
var quietServerCreation = false
var dockerSubnet = ""

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "wireport server commands",
	Long:  `Manage connected wireport server nodes and create join-requests for connecting new servers to the wireport network`,
}

var NewServerCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a server to wireport network",
	Long:  `Create a new join-request for connecting a server to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command)`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServerNew(cmd.OutOrStdout(), cmd.ErrOrStderr(), forceServerCreation, quietServerCreation, dockerSubnet)
	},
}

var StartServerCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the wireport server",
	Long:  `Start the wireport server. This command is only relevant for server nodes after they joined the network.`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServerStart(cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var StatusServerCmd = &cobra.Command{
	Use:   "status [username@hostname[:port]]",
	Short: "Check wireport server node status",
	Long:  `Check the status of wireport server node: SSH connection, Docker installation, and wireport server status.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Build credentials from positional argument or flags
		creds, err := buildSSHCredentials(cmd, args, false, false)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.ServerStatus(creds, cmd.OutOrStdout())
	},
}

var UpServerCmd = &cobra.Command{
	Use:   "up",
	Short: "Up a wireport server node",
	Long:  `Up a wireport server node. This command is only relevant for server nodes after they joined the network.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, false, false)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		if dockerSubnet != "" {
			// validate the subnet format
			_, err := types.ParseIPNetMarshable(dockerSubnet, true)

			if err != nil {
				cmd.PrintErrf("Failed to parse Docker subnet: %v\n", err)
				return
			}
		}

		commandsService.ServerUp(creds, cmd.OutOrStdout(), cmd.ErrOrStderr(), dockerSubnet)
	},
}

var DownServerCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop wireport server node",
	Long:  `Stop wireport server node. This command is only relevant for server nodes after they joined the network.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var creds *ssh.Credentials

		if len(args) > 0 {
			var err error
			creds, err = buildSSHCredentials(cmd, args, false, false)

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
	Use:   "upgrade",
	Short: "Upgrade a server",
	Long:  `Upgrade a server. This command is only relevant for server nodes after they joined the network.`,
	Run: func(cmd *cobra.Command, args []string) {
		creds, err := buildSSHCredentials(cmd, args, false, false)

		if err != nil {
			cmd.PrintErrf("❌ Error: %v\n", err)
			return
		}

		commandsService.ServerUpgrade(creds, cmd.OutOrStdout(), cmd.ErrOrStderr())
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

	StatusServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")

	UpServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
	UpServerCmd.Flags().StringVar(&dockerSubnet, "docker-subnet", "", "Specify a custom Docker subnet for the server (e.g. 172.20.0.0/16)")

	DownServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")

	UpgradeServerCmd.Flags().String("ssh-key-path", "", "Path to SSH private key file (for passwordless authentication)")
}
