package publicservices

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type PublicServiceParam struct {
	ParamType  PublicServiceParamType `json:"param_type"`
	ParamValue string                 `json:"param_value"`
}
type PublicServiceParamType string

const (
	PublicServiceParamTypeCaddyFreeText PublicServiceParamType = "caddyFreeTextParam"
)

type PublicService struct {
	LocalProtocol string `gorm:"type:text;not null"`    // http, https, udp, tcp
	LocalHost     string `gorm:"type:text;not null"`    // domain, ip
	LocalPort     uint16 `gorm:"type:integer;not null"` // port

	PublicProtocol string `gorm:"type:text;primaryKey;uniqueIndex:idx_public_service"`    // http, https, udp, tcp
	PublicHost     string `gorm:"type:text;primaryKey;uniqueIndex:idx_public_service"`    // domain:port
	PublicPort     uint16 `gorm:"type:integer;primaryKey;uniqueIndex:idx_public_service"` // port

	Params []PublicServiceParam `gorm:"type:text;serializer:json;not null;default:[]"`

	CreatedAt time.Time `gorm:"type:timestamp;not null"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null"`
}

func formatBlockParams(blockParams []PublicServiceParam, levelSpacesMain int, levelSpacesClosing int) string {
	if len(blockParams) == 0 {
		return ""
	}

	blockParamsList := []string{}

	for _, blockParam := range blockParams {
		blockParamsList = append(blockParamsList, fmt.Sprintf(strings.Repeat(" ", levelSpacesMain)+"%s", blockParam.ParamValue))
	}

	return fmt.Sprintf("{\n%s\n%s}", strings.Join(blockParamsList, "\n"), strings.Repeat(" ", levelSpacesClosing))
}

func (s *PublicService) AsCaddyConfigEntry(gatewayPublicIP string) (result string, err error) {
	if (s.LocalProtocol == "udp" && s.PublicProtocol == "tcp") ||
		(s.LocalProtocol == "tcp" && s.PublicProtocol == "udp") {
		return "", fmt.Errorf("for layer 4, local protocol and public protocol must be the same (udp -> udp or tcp -> tcp)")
	}

	if s.LocalHost == "" {
		return "", fmt.Errorf("local host cannot be empty")
	}

	if s.PublicHost == "" {
		return "", fmt.Errorf("public host cannot be empty")
	}

	publicHost := s.PublicHost

	if publicHost == gatewayPublicIP {
		// caddy won't see the network interface for the gateway public IP from inside docker containers, so we use 0.0.0.0
		publicHost = "0.0.0.0"
	}

	localHost := s.LocalHost

	if localHost == gatewayPublicIP {
		// caddy won't see the network interface for the gateway public IP from inside docker containers, so we use 0.0.0.0
		localHost = "0.0.0.0"
	}

	result = fmt.Sprintf("# service publication: %s://%s:%d (public) -> %s://%s:%d (local)", s.PublicProtocol, publicHost, s.PublicPort, s.LocalProtocol, localHost, s.LocalPort)

	switch s.PublicProtocol {
	case "https", "http":
		// fallback option for standard ports (80 and 443)
		publicHostname := fmt.Sprintf("%s://%s", s.PublicProtocol, publicHost)
		publicHostnameIsIP := false

		if ip := net.ParseIP(publicHost); ip != nil {
			publicHostnameIsIP = true
		}

		if s.PublicProtocol == "https" {
			if publicHostnameIsIP {
				return "", fmt.Errorf("https on ip address is not supported")
			}

			if s.PublicPort != 443 {
				publicHostname = fmt.Sprintf("%s://%s:%d", s.PublicProtocol, publicHost, s.PublicPort)
			}
		} else if s.PublicProtocol == "http" && s.PublicPort != 80 {
			// if hostname is a dns-name - use full entry, otherwise - use just the port

			if publicHostnameIsIP {
				// ip
				publicHostname = fmt.Sprintf(":%d", s.PublicPort)
			} else {
				// dns name
				publicHostname = fmt.Sprintf("%s://%s:%d", s.PublicProtocol, publicHost, s.PublicPort)
			}
		}

		reverseProxy := strings.TrimSpace(fmt.Sprintf("reverse_proxy %s://%s:%d %s", s.LocalProtocol, localHost, s.LocalPort, formatBlockParams(s.Params, 8, 4)))

		result = fmt.Sprintf(`
%s {
    %s
}
`, publicHostname, reverseProxy)
	case "udp", "tcp":
		upstream := strings.TrimSpace(fmt.Sprintf("upstream %s/%s:%d %s", s.LocalProtocol, localHost, s.LocalPort, formatBlockParams(s.Params, 16, 12)))

		result = fmt.Sprintf(`
        %s/%s:%d {
            route {
                proxy {
                    %s
                }
            }
        }
`, s.PublicProtocol, publicHost, s.PublicPort, upstream)
	}

	return strings.TrimRight(result, "\t "), nil
}
