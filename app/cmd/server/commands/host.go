package commands

import (
	"fmt"
	"net"
	"net/http"

	"wireport/cmd/server/config"
	"wireport/internal/logger"
	"wireport/internal/nodes/types"
	"wireport/internal/routes"

	"github.com/spf13/cobra"
)

var HostStartConfigureOnly bool = false

var HostCmd = &cobra.Command{
	Use:   "host",
	Short: "wireport host commands",
	Long:  `Manage wireport host node: configure the host node and start the wireport host node`,
}

var StartHostCmd = &cobra.Command{
	Use:   "start",
	Short: "Start wireport in host mode",
	Long:  `Start wireport in host mode. It will handle network connections and state management.`,
	Run: func(cmd *cobra.Command, args []string) {
		router := routes.Router(dbInstance)

		publicIP, err := join_requests_service.GetPublicIP()

		if err != nil {
			logger.Fatal("Failed to get public IP: %v", err)
			return
		}

		serverError := make(chan error, 1)

		if !HostStartConfigureOnly {
			go func() {
				if err := http.ListenAndServe(fmt.Sprintf(":%d", config.Config.ControlServerPort), router); err != nil {
					serverError <- err
				}
			}()
		}

		hostNode, err := nodes_repository.EnsureHostNode(types.IPMarshable{
			IP: net.ParseIP(*publicIP),
		}, config.Config.WGPublicPort)

		if err != nil {
			logger.Error("wireport host node start failed: %v", err)
			logger.Fatal("Failed to ensure host node: %v", err)
			return
		}

		publicServices := public_services_repository.GetAll()

		err = hostNode.SaveConfigs(publicServices)

		if err != nil {
			logger.Fatal("Failed to save configs: %v", err)
			return
		}

		if !HostStartConfigureOnly {
			logger.Info("wireport server has started on host: %s", *hostNode.WGPublicIp)
		} else {
			logger.Info("wireport has been configured on the host: %s", *hostNode.WGPublicIp)
		}

		if !HostStartConfigureOnly {
			// Block on the server error channel
			if err := <-serverError; err != nil {
				logger.Fatal("Server error: %v", err)
			}
		}
	},
}

func init() {
	HostCmd.AddCommand(StartHostCmd)
	StartHostCmd.Flags().BoolVar(&HostStartConfigureOnly, "configure", false, "Configure wireport in host mode without making it available for external connections")
}
