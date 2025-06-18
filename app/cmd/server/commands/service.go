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
var paramValue string

var ServiceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage public services",
	Long:  `Manage public services that are exposed to the internet`,
}

var ParamsServiceCmd = &cobra.Command{
	Use:   "params",
	Short: "Manage params of the service",
	Long:  "Manage params of the service (e.g., caddyfile directives or so)",
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

var ListServiceCmd = &cobra.Command{
	Use:   "list",
	Short: "List all published services",
	Long:  `List all published services (public and local addresses)`,
	Run: func(cmd *cobra.Command, args []string) {
		if !nodes_repository.IsCurrentNodeHost() {
			cmd.Printf("This command can only be used on a host node\n")
			return
		}

		services := public_services_repository.GetAll()

		cmd.Printf("PUBLIC\tLOCAL\n")

		for _, service := range services {
			cmd.Printf("%s://%s:%d\t%s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort, service.LocalProtocol, service.LocalHost, service.LocalPort)
		}
	},
}

var NewParamsServiceCmd = &cobra.Command{
	Use:   "new",
	Short: "Add a new parameter to a public service",
	Long:  `Add a new parameter to a public service. Parameters are used for parametrization of the service (e.g., setting up custom headers for an http/https reverse proxy and so on)`,
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

		updated := public_services_repository.AddParam(*publicProtocol, *publicHost, *publicPort, public_services.PublicServiceParamTypeCaddyFreeText, paramValue)

		if updated {
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

			cmd.Printf("Parameter added successfully\n")
		} else {
			cmd.Printf("Parameter was not added (probably already exists)\n")
		}
	},
}

var RemoveParamsServiceCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a parameter from a public service",
	Long:  `Remove a parameter from a public service. Parameters are used for parametrization of the service (e.g., setting up custom headers for an http/https reverse proxy and so on)`,
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

		removed := public_services_repository.RemoveParam(*publicProtocol, *publicHost, *publicPort, public_services.PublicServiceParamTypeCaddyFreeText, paramValue)

		if removed {
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

			cmd.Printf("Parameter removed successfully\n")
		} else {
			cmd.Printf("Parameter was not removed (probably not found)\n")
		}
	},
}

var ListParamsServiceCmd = &cobra.Command{
	Use:   "list",
	Short: "List all parameters of a public service",
	Long:  `List all parameters of a public service (e.g., caddyfile directives)`,
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

		service, err := public_services_repository.Get(*publicProtocol, *publicHost, *publicPort)

		if err != nil {
			cmd.Printf("Error getting service: %v\n", err)
			return
		}

		cmd.Printf("Params of %s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort)

		for _, param := range service.Params {
			cmd.Printf("%s\n", param.ParamValue)
		}

		cmd.Printf("\n")
	},
}

func init() {
	PublishServiceCmd.Flags().StringVarP(&local, "local", "l", "", "Local address of the service (e.g. tcp://localhost:4000)")
	PublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")

	UnpublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")

	NewParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")
	NewParamsServiceCmd.Flags().StringVar(&paramValue, "param-value", "", "Value of the parameter to add (e.g. 'header_up X-Tenant-Hostname {http.request.host}', 'dial_timeout 5s' and other valid caddy directives for reverse proxy and/or layer 4 Caddyfile directives)")

	RemoveParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")
	RemoveParamsServiceCmd.Flags().StringVar(&paramValue, "param-value", "", "Value of the parameter to remove (e.g. 'header_up X-Tenant-Hostname {http.request.host}', 'dial_timeout 5s' and other valid caddy directives for reverse proxy and/or layer 4 Caddyfile directives)")

	ListParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://public:4434)")

	ParamsServiceCmd.AddCommand(NewParamsServiceCmd)
	ParamsServiceCmd.AddCommand(RemoveParamsServiceCmd)
	ParamsServiceCmd.AddCommand(ListParamsServiceCmd)

	ServiceCmd.AddCommand(PublishServiceCmd)
	ServiceCmd.AddCommand(UnpublishServiceCmd)
	ServiceCmd.AddCommand(ParamsServiceCmd)
	ServiceCmd.AddCommand(ListServiceCmd)
}
