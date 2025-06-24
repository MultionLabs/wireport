package commands

import (
	"wireport/internal/commands"
	join_requests "wireport/internal/join-requests"
	"wireport/internal/nodes"
	public_services "wireport/internal/public-services"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	dbInstance                 *gorm.DB
	nodes_repository           *nodes.Repository
	join_requests_repository   *join_requests.Repository
	public_services_repository *public_services.Repository
	commandsService            *commands.Service
)

func RegisterCommands(rootCmd *cobra.Command, db *gorm.DB) {
	dbInstance = db
	nodes_repository = nodes.NewRepository(db)
	join_requests_repository = join_requests.NewRepository(db)
	public_services_repository = public_services.NewRepository(db)
	commandsService = &commands.Service{}

	rootCmd.AddCommand(HostCmd)
	rootCmd.AddCommand(ServerCmd)
	rootCmd.AddCommand(ClientCmd)
	rootCmd.AddCommand(JoinCmd)
	rootCmd.AddCommand(ServiceCmd)
}
