package commands

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"wireport/internal/publicservices"
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
	Run: func(cmd *cobra.Command, _ []string) {
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

		commandsService.ServicePublish(cmd.OutOrStdout(), cmd.ErrOrStderr(), *localProtocol, *localHost, *localPort, *publicProtocol, *publicHost, *publicPort)
	},
}

var UnpublishServiceCmd = &cobra.Command{
	Use:   "unpublish",
	Short: "Unpublish a public service",
	Long:  `Unpublish a public service and make it no longer accessible from the internet`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		commandsService.ServiceUnpublish(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort)
	},
}

var ListServiceCmd = &cobra.Command{
	Use:   "list",
	Short: "List all published services",
	Long:  `List all published services (public and local addresses)`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServiceList(cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var NewParamsServiceCmd = &cobra.Command{
	Use:   "new",
	Short: "Add a new parameter to a public service",
	Long:  `Add a new parameter to a public service. Parameters are used for parametrization of the service (e.g., setting up custom headers for an http/https reverse proxy and so on)`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		commandsService.ServiceParamNew(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort, publicservices.PublicServiceParamTypeCaddyFreeText, paramValue)
	},
}

var RemoveParamsServiceCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a parameter from a public service",
	Long:  `Remove a parameter from a public service. Parameters are used for parametrization of the service (e.g., setting up custom headers for an http/https reverse proxy and so on)`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		commandsService.ServiceParamRemove(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort, publicservices.PublicServiceParamTypeCaddyFreeText, paramValue)
	},
}

var ListParamsServiceCmd = &cobra.Command{
	Use:   "list",
	Short: "List all parameters of a public service",
	Long:  `List all parameters of a public service (e.g., caddyfile directives)`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := parseAddress(public)

		if err != nil {
			cmd.Printf("Error parsing public address: %v\n", err)
			return
		}

		commandsService.ServiceParamList(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort)
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
