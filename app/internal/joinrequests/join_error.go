package joinrequests

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"wireport/cmd/server/config"
)

// FormatJoinError turns a join API failure into actionable guidance, with emphasis on
// firewall and connectivity issues that commonly block server bootstrap
func FormatJoinError(err error, gatewayAddress string) string {
	if err == nil {
		return ""
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Failed to reach the wireport gateway at %s.\n", gatewayAddress)
	fmt.Fprintf(&b, "Underlying error: %v\n\n", err)

	b.WriteString(diagnoseJoinError(err))
	b.WriteString(ConnectivityRequirementsText())
	fmt.Fprintf(&b, "Full port and firewall requirements: %s\n\n", config.Config.DocumentationURL)
	b.WriteString("After fixing firewall rules, the server container will retry joining automatically — a container restart is not required.\n")

	return b.String()
}

func diagnoseJoinError(err error) string {
	lower := strings.ToLower(err.Error())

	switch {
	case containsAny(lower, "connection refused", "connect: connection refused"):
		return "Diagnosis: nothing is accepting connections on the gateway control port. " +
			"Ensure the gateway container is running, then check firewalls on both hosts: " +
			"outbound TCP " + portStr(config.Config.ControlServerPort) + " from the server and " +
			"inbound TCP " + portStr(config.Config.ControlServerPort) + " on the gateway.\n\n"

	case containsAny(lower, "i/o timeout", "context deadline exceeded", "connection timed out", "timeout"):
		return "Diagnosis: the connection to the gateway timed out. This usually means a firewall (on the server host, gateway host, or a cloud security group) is dropping traffic. " +
			"Verify outbound rules on this server and inbound rules on the gateway for the ports listed below.\n\n"

	case isSyscallErr(err, syscall.ENETUNREACH, syscall.EHOSTUNREACH):
		return "Diagnosis: the network route to the gateway is unreachable. Check routing, security groups, and host firewalls on both sides.\n\n"

	case containsAny(lower, "no such host", "name or service not known"):
		return "Diagnosis: the gateway hostname could not be resolved. Verify DNS and that the gateway public IP/hostname in the join token is correct.\n\n"

	case containsAny(lower, "tls:", "x509:", "certificate"):
		return "Diagnosis: TLS handshake with the gateway failed. If you recently reinstalled the gateway, generate a new join token and bootstrap the server again.\n\n"

	default:
		return "Diagnosis: could not complete the join handshake with the gateway. Check gateway availability and firewall rules on both the server and gateway hosts.\n\n"
	}
}

// ConnectivityRequirementsText describes firewall and port rules on both the server and gateway.
func ConnectivityRequirementsText() string {
	return fmt.Sprintf(
		"Check firewall rules on BOTH the server and gateway hosts (and any cloud security groups):\n\n"+
			"Server host:\n"+
			"  - Outbound TCP %d → gateway (join / control API)\n"+
			"  - Outbound UDP %d → gateway (WireGuard tunnel)\n\n"+
			"Gateway host:\n"+
			"  - Inbound TCP %d from the server (join / control API)\n"+
			"  - Inbound UDP %d (WireGuard; also 80/tcp and 443/tcp for published services)\n\n"+
			"If the gateway and server run on the same machine, both sets of rules apply on that host.\n\n",
		config.Config.ControlServerPort,
		config.Config.WGPublicPort,
		config.Config.ControlServerPort,
		config.Config.WGPublicPort,
	)
}

func portStr(port uint16) string {
	return fmt.Sprintf("%d", port)
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func isSyscallErr(err error, codes ...syscall.Errno) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return isSyscallErr(opErr.Err, codes...)
		}
		return false
	}
	for _, code := range codes {
		if errno == code {
			return true
		}
	}
	return false
}
