package commands

import (
	"github.com/spf13/cobra"
)

var JoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join the wireport network",
	Long:  `Join the wireport network using a join-request token, provided by the 'wireport server new' command`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			cmd.PrintErrf("Provide a join token\n")
			return
		}

		joinToken := args[0]

		if joinToken == "" {
			cmd.PrintErrf("Provide a join token\n")
			return
		}

		commandsService.Join(nodes_repository, cmd.OutOrStdout(), cmd.ErrOrStderr(), joinToken)
	},
}
