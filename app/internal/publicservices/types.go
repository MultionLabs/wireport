package publicservices

import (
	"fmt"
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

func (s *PublicService) AsCaddyConfigEntry() (result string, err error) {
	if (s.LocalProtocol == "udp" && s.PublicProtocol == "tcp") ||
		(s.LocalProtocol == "tcp" && s.PublicProtocol == "udp") {
		return "", fmt.Errorf("for layer 4, local protocol and public protocol must be the same (udp -> udp or tcp -> tcp)")
	}

	result = fmt.Sprintf("# service publication: %s://%s:%d (public) -> %s://%s:%d (local)", s.PublicProtocol, s.PublicHost, s.PublicPort, s.LocalProtocol, s.LocalHost, s.LocalPort)

	switch s.PublicProtocol {
	case "https", "http":
		publicHostname := fmt.Sprintf("%s://%s", s.PublicProtocol, s.PublicHost)

		if s.PublicProtocol == "https" && s.PublicPort != 443 {
			publicHostname = fmt.Sprintf("%s:%d", s.PublicHost, s.PublicPort)
		} else if s.PublicProtocol == "http" && s.PublicPort != 80 {
			publicHostname = fmt.Sprintf("%s:%d", s.PublicHost, s.PublicPort)
		}

		reverseProxy := strings.TrimSpace(fmt.Sprintf("reverse_proxy %s://%s:%d %s", s.LocalProtocol, s.LocalHost, s.LocalPort, formatBlockParams(s.Params, 8, 4)))

		result = fmt.Sprintf(`
%s {
    %s
}
`, publicHostname, reverseProxy)
	case "udp", "tcp":
		upstream := strings.TrimSpace(fmt.Sprintf("upstream %s/%s:%d %s", s.LocalProtocol, s.LocalHost, s.LocalPort, formatBlockParams(s.Params, 16, 12)))

		result = fmt.Sprintf(`
        %s/%s:%d {
            route {
                proxy {
                    %s
                }
            }
        }
`, s.PublicProtocol, s.PublicHost, s.PublicPort, upstream)
	}

	return strings.TrimRight(result, "\t "), nil
}
