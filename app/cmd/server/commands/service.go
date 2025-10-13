package commands

import (
	"github.com/spf13/cobra"

	"wireport/cmd/server/config"
	"wireport/internal/publicservices"
	"wireport/internal/utils"
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

var PublishServiceCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish a new public service",
	Long: `Publish a new public service that should be exposed to the Internet.
	
	Supported protocols: tcp, udp, http, https
	Supported public ports: 80, 443, 32420-32421 (tcp+udp; should be open in the firewall on the gateway node)
	Supported local ports: any
	Supported local hosts: private IP addresses (e.g. 10.0.0.2) of CLIENT and SERVER nodes on wireport network
	
	Example:

	wireport service publish --local http://10.0.0.2:4000 --public https://demo.server.com:443
	wireport service publish --local tcp://10.0.0.2:4000 --public tcp://0.0.0.0:443`,
	Run: func(cmd *cobra.Command, _ []string) {
		localProtocol, localHost, localPort, err := utils.ParseAddress(local)

		if err != nil {
			cmd.Printf("❌ Error: local address parsing failed: %v\n", err)
			return
		}

		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(public)

		if err != nil {
			cmd.Printf("❌ Error: public address parsing failed: %v\n", err)
			return
		}

		if *publicPort == config.Config.ControlServerPort {
			cmd.Printf("❌ Error: port %d is reserved for wireport control plane and cannot be used for publishing services\n", config.Config.ControlServerPort)
			return
		}

		if (*localProtocol == "tcp" && *publicProtocol != "tcp") ||
			(*localProtocol == "udp" && *publicProtocol != "udp") {
			cmd.Printf("❌ Error: local protocol and public protocol must be the same for layer 4 services (tcp -> tcp or udp -> udp)\n")
			return
		}

		currentNode, err := nodesRepository.GetCurrentNode()

		if err != nil {
			cmd.Printf("❌ Error: failed to get current node: %v\n", err)
			return
		}

		commandsService.ServicePublish(cmd.OutOrStdout(), cmd.ErrOrStderr(), &currentNode.ID, *localProtocol, *localHost, *localPort, *publicProtocol, *publicHost, *publicPort)
	},
}

var UnpublishServiceCmd = &cobra.Command{
	Use:   "unpublish",
	Short: "Unpublish a public service",
	Long: `Unpublish a public service and make it no longer accessible from the Internet.
	
	Example:

	wireport service unpublish --public tcp://140.120.10.10:443`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(public)

		if err != nil {
			cmd.Printf("❌ Error: public address parsing failed: %v\n", err)
			return
		}

		commandsService.ServiceUnpublish(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort)
	},
}

var ListServiceCmd = &cobra.Command{
	Use:   "list",
	Short: "List all published services",
	Long:  `List all published services.`,
	Run: func(cmd *cobra.Command, _ []string) {
		commandsService.ServiceList(cmd.OutOrStdout(), cmd.ErrOrStderr())
	},
}

var NewParamsServiceCmd = &cobra.Command{
	Use:   "new",
	Short: "Add a new parameter to a public service",
	Long:  `Add a new parameter to a public service. Parameters are used for parametrization of the service (e.g., setting up custom headers for an http/https reverse proxy or layer 4 Caddyfile directives)`,
	Run: func(cmd *cobra.Command, _ []string) {
		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(public)

		if err != nil {
			cmd.Printf("❌ Error: public address parsing failed: %v\n", err)
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
		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(public)

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
		publicProtocol, publicHost, publicPort, err := utils.ParseAddress(public)

		if err != nil {
			cmd.Printf("❌ Error: public address parsing failed: %v\n", err)
			return
		}

		commandsService.ServiceParamList(cmd.OutOrStdout(), cmd.ErrOrStderr(), *publicProtocol, *publicHost, *publicPort)
	},
}

func init() {
	PublishServiceCmd.Flags().StringVarP(&local, "local", "l", "", "Local address of the service (e.g. tcp://localhost:4000)")
	PublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://140.120.10.10:32420)")

	UnpublishServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://140.120.10.10:32420)")

	NewParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://140.120.10.10:32420)")
	NewParamsServiceCmd.Flags().StringVar(&paramValue, "param-value", "", "Value of the parameter to add (e.g. 'header_up X-Tenant-Hostname {http.request.host}', 'dial_timeout 5s' and other valid caddy directives for reverse proxy and/or layer 4 Caddyfile directives)")

	RemoveParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://140.120.10.10:32420)")
	RemoveParamsServiceCmd.Flags().StringVar(&paramValue, "param-value", "", "Value of the parameter to remove (e.g. 'header_up X-Tenant-Hostname {http.request.host}', 'dial_timeout 5s' and other valid caddy directives for reverse proxy and/or layer 4 Caddyfile directives)")

	ListParamsServiceCmd.Flags().StringVarP(&public, "public", "p", "", "Public address of the service (e.g. tcp://140.120.10.10:32420)")

	ParamsServiceCmd.AddCommand(NewParamsServiceCmd)
	ParamsServiceCmd.AddCommand(RemoveParamsServiceCmd)
	ParamsServiceCmd.AddCommand(ListParamsServiceCmd)

	ServiceCmd.AddCommand(PublishServiceCmd)
	ServiceCmd.AddCommand(UnpublishServiceCmd)
	ServiceCmd.AddCommand(ParamsServiceCmd)
	ServiceCmd.AddCommand(ListServiceCmd)
}
