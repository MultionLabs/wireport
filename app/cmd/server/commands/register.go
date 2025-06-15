package commands

import (
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
	join_requests_service      *join_requests.APIService
	public_services_repository *public_services.PublicServiceRepository
)

func RegisterCommands(rootCmd *cobra.Command, db *gorm.DB) {
	dbInstance = db
	nodes_repository = nodes.NewRepository(db)
	join_requests_repository = join_requests.NewRepository(db)
	join_requests_service = join_requests.NewAPIService()
	public_services_repository = public_services.NewRepository(db)

	rootCmd.AddCommand(HostCmd)
	rootCmd.AddCommand(ServerCmd)
	rootCmd.AddCommand(ClientCmd)
	rootCmd.AddCommand(JoinCmd)
	rootCmd.AddCommand(ServiceCmd)
}
