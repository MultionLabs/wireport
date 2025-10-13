package utils

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func GetPublicIP() (*string, error) {
	resp, err := http.Get("https://ipinfo.io/ip")

	if err != nil {
		return nil, fmt.Errorf("failed to get node public ip (required for gateway setup): %w", err)
	}

	defer resp.Body.Close()

	publicIP, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read node public ip (required for gateway setup): %w", err)
	}

	publicIPString := string(publicIP)

	return &publicIPString, nil
}

func ParseAddress(addr string) (protocol, host *string, port *uint16, err error) {
	addr = strings.TrimSpace(addr)

	if addr == "" {
		return nil, nil, nil, errors.New("valid host value is required")
	}

	u, err := url.Parse(addr)

	if err != nil {
		return nil, nil, nil, err
	}

	protocolString := u.Scheme

	if protocolString == "" {
		return nil, nil, nil, errors.New("protocol is required")
	}

	lowerProtocolString := strings.ToLower(protocolString)
	protocol = &lowerProtocolString

	switch *protocol {
	case "tcp", "udp", "http", "https":
		// supported, do nothing
	default:
		return nil, nil, nil, errors.New("unsupported protocol")
	}

	hostname := u.Hostname()

	if hostname == "" {
		return nil, nil, nil, errors.New("host is required")
	}

	portString := u.Port()

	host = &hostname

	if portString == "" {
		switch *protocol {
		case "http":
			portString = "80"
		case "https":
			portString = "443"
		default:
			return nil, nil, nil, errors.New("port is required for layer 4 services")
		}
	}

	portInt, err := strconv.Atoi(portString)

	if err != nil || portInt < 1 || portInt > 65535 {
		return nil, nil, nil, errors.New("invalid port")
	}

	portUint16 := uint16(portInt)

	port = &portUint16

	return protocol, host, port, nil
}
