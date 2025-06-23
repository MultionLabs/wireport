package commands

import (
	"github.com/spf13/cobra"
)

var forceClientCreation bool = false
var quietClientCreation bool = false

var ClientCmd = &cobra.Command{
	Use:   "client",
	Short: "wireport client commands",
	Long:  `Manage wireport client nodes: create a wireguard configuration for the client`,
}

var NewClientCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a client to wireport network",
	Long:  `Create a new join-request for connecting a client to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command)`,
	Run: func(cmd *cobra.Command, args []string) {
		commandsService.ClientNew(nodes_repository, join_requests_repository, public_services_repository, cmd.OutOrStdout(), cmd.ErrOrStderr(), forceClientCreation, quietClientCreation)
	},
}

func init() {
	NewClientCmd.Flags().BoolVarP(&forceClientCreation, "force", "f", false, "Force create a client node without generating a join request")
	NewClientCmd.Flags().BoolVarP(&quietClientCreation, "quiet", "q", false, "Quiet mode, don't print any output except for the join request token")

	ClientCmd.AddCommand(NewClientCmd)
}
