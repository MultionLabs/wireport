package commands

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	public_services "wireport/internal/public-services"
	"wireport/internal/terminal"
)

var local string
var public string

var ServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage public services",
	Long:  `Manage public services that are exposed to the internet`,
}

func parseAddress(addr string) (protocol, host *string, port *uint16, err error) {
	u, err := url.Parse(addr)

	if err != nil {
		return nil, nil, nil, err
	}

	protocol = &u.Scheme

	if *protocol != "tcp" && *protocol != "udp" && *protocol != "http" && *protocol != "https" {
		return nil, nil, nil, errors.New("unsupported protocol")
	}

	hostname := u.Hostname()
	portString := u.Port()

	host = &hostname

	if portString == "" {
		switch *protocol {
		case "http":
			portString = "80"
		case "https":
			portString = "443"
		}
	}

	portInt, err := strconv.Atoi(portString)
	portUint16 := uint16(portInt)

	if err != nil {
		return nil, nil, nil, err
	}

	port = &portUint16

	return protocol, host, port, nil
}

var PublishServiceCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a new public service",
	Long:  `Publish a new public service that should be exposed to the internet`,
	Run: func(cmd *cobra.Command, args []string) {
		if !nodes_repository.IsCurrentNodeHost() {
			cmd.Printf("This command can only be used on a host node\n")
			return
		}

		localProtocol, localHost, localPort, err := parseAddress(local)

		if err != nil {
			cmd.Printf("Error parsing local address: %v\n", err)
			return
		}

		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		err = public_services_repository.Save(&public_services.PublicService{
			LocalProtocol:  *localProtocol,
			LocalHost:      *localHost,
			LocalPort:      *localPort,
			PublicProtocol: *publicProtocol,
			PublicHost:     *publicHost,
			PublicPort:     *publicPort,
		})

		if err != nil {
			cmd.Printf("Error creating public service: %v\n", err)
			return
		}

		hostNode, err := nodes_repository.GetHostNode()

		if err != nil {
			cmd.Printf("Error getting host node: %v\n", err)
			return
		}

		err = hostNode.SaveConfigs(public_services_repository.GetAll(), false)

		if err != nil {
			cmd.Printf("Error saving host node configs: %v\n", err)
			return
		}

		err = terminal.RestartServices(false, false, true)

		if err != nil {
			cmd.Printf("Error restarting services: %v\n", err)
			return
		}

		cmd.Printf("Service %s://%s:%d is now published on\n\n\t\t%s://%s:%d\n\n", *localProtocol, *localHost, *localPort, *publicProtocol, *publicHost, *publicPort)
	},
}

var UnpublishServiceCmd = &cobra.Command{
	Use:   "unpublish",
	Short: "Unpublish a public service",
	Long:  `Unpublish a public service and make it no longer accessible from the internet`,
	Run: func(cmd *cobra.Command, args []string) {
		if !nodes_repository.IsCurrentNodeHost() {
			cmd.Printf("This command can only be used on a host node\n")
			return
		}

		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		unpublished := public_services_repository.Delete(*publicProtocol, *publicHost, *publicPort)

		if unpublished {
			hostNode, err := nodes_repository.GetHostNode()

			if err != nil {
				cmd.Printf("Error getting host node: %v\n", err)
				return
			}

			err = hostNode.SaveConfigs(public_services_repository.GetAll(), false)

			if err != nil {
				cmd.Printf("Error saving host node configs: %v\n", err)
				return
			}

			err = terminal.RestartServices(false, false, true)

			if err != nil {
				cmd.Printf("Error restarting services: %v\n", err)
				return
			}

			cmd.Printf("\nService %s://%s:%d is now unpublished\n", *publicProtocol, *publicHost, *publicPort)
		} else {
			cmd.Printf("\nService %s://%s:%d was not found or was already unpublished\n", *publicProtocol, *publicHost, *publicPort)
		}
	},
}

func init() {
	PublishServiceCmd.Flags().StringVarP(&local, "local", "l", "", "Local address of the service (e.g. tcp://localhost:4000)")
	PublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")

	UnpublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")

	ServiceCmd.AddCommand(PublishServiceCmd)
	ServiceCmd.AddCommand(UnpublishServiceCmd)
}
