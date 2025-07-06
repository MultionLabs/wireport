package commands

import (
	"github.com/spf13/cobra"
)

var JoinPostponed bool

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

		if JoinPostponed {
			_, err := joinTokensRepository.Create(joinToken)

			if err != nil {
				cmd.PrintErrf("Failed to create join token: %v\n", err)
				return
			}

			cmd.Printf("Join token has been saved and will be applied on the next server start\n")
		} else {
			commandsService.Join(cmd.OutOrStdout(), cmd.ErrOrStderr(), joinToken)
		}
	},
}

func init() {
	JoinCmd.Flags().BoolVar(&JoinPostponed, "postponed", false, "Postpone the join until the next server start (useful for server setup)")
}
