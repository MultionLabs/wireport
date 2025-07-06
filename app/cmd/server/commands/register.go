package commands

import (
	"wireport/internal/commands"
	"wireport/internal/joinrequests"
	"wireport/internal/jointokens"
	"wireport/internal/nodes"
	"wireport/internal/publicservices"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	dbInstance               *gorm.DB
	nodesRepository          *nodes.Repository
	joinRequestsRepository   *joinrequests.Repository
	publicServicesRepository *publicservices.Repository
	joinTokensRepository     *jointokens.Repository
	commandsService          *commands.Service
)

func RegisterCommands(rootCmd *cobra.Command, db *gorm.DB) {
	dbInstance = db
	nodesRepository = nodes.NewRepository(db)
	joinRequestsRepository = joinrequests.NewRepository(db)
	publicServicesRepository = publicservices.NewRepository(db)
	joinTokensRepository = jointokens.NewRepository(db)
	commandsService = &commands.Service{
		LocalCommandsService: commands.LocalCommandsService{
			NodesRepository:          nodesRepository,
			PublicServicesRepository: publicServicesRepository,
			JoinRequestsRepository:   joinRequestsRepository,
			JoinTokensRepository:     joinTokensRepository,
		},
		NodesRepository:          nodesRepository,
		PublicServicesRepository: publicServicesRepository,
		JoinRequestsRepository:   joinRequestsRepository,
	}

	rootCmd.AddCommand(GatewayCmd)
	rootCmd.AddCommand(ServerCmd)
	rootCmd.AddCommand(ClientCmd)
	rootCmd.AddCommand(JoinCmd)
	rootCmd.AddCommand(ServiceCmd)
}
