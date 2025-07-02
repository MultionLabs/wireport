package commands

import (
	"github.com/spf13/cobra"
)

var joinRequestClientCreation = false
var quietClientCreation = false
var waitClientCreation = false

var ClientCmd = &cobra.Command{
	Use:   "client",
	Short: "wireport client commands",
	Long:  `Manage wireport client nodes: create a wireguard configuration for the client`,
}

var NewClientCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new join-request for connecting a client to wireport network",
	Long:  `Create a new join-request for connecting a client to wireport network. The join-request will generate a token that can be used to join the network (see 'wireport join' command)`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ClientNew(cmd.OutOrStdout(), cmd.ErrOrStderr(), joinRequestClientCreation, quietClientCreation, waitClientCreation)
	},
}

var ListClientCmd = &cobra.Command{
	Use:   "list",
	Short: "List all clients",
	Long:  `List all clients that are connected to the wireport network`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ClientList(nil, cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

func init() {
	NewClientCmd.Flags().BoolVarP(&joinRequestClientCreation, "join-request", "j", false, "Create a join request for connecting a client to wireport network (by default, a client is created, bypassing the join request)")
	NewClientCmd.Flags().BoolVarP(&quietClientCreation, "quiet", "q", false, "Quiet mode, don't print any output except for the join request token")
	NewClientCmd.Flags().BoolVarP(&waitClientCreation, "wait", "w", false, "Wait for the client to be created (will check periodically whether there's a gateway node available and if a client can be created then)")

	ClientCmd.AddCommand(NewClientCmd)
	ClientCmd.AddCommand(ListClientCmd)
}
