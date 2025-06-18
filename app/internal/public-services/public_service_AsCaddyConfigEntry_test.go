package public_services

import (
	"strings"
	"testing"
)

func removeSpaces(s string) string {
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// layer 7, local address variations

func TestPublicService_AsCaddyConfigEntry_Layer7_With_No_BlockParams(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:8080
}
`
	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_1_BlockParam(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{{ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "header_up X-Tenant-Hostname {http.request.host}"}},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:8080 {
        header_up X-Tenant-Hostname {http.request.host}
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_Multiple_BlockParams(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{{ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "header_up X-Tenant-Hostname {http.request.host}"}, {ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "header_up X-Tenant-Port {http.request.port}"}},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:8080 {
        header_up X-Tenant-Hostname {http.request.host}
        header_up X-Tenant-Port {http.request.port}
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_http_Standard_Local_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      80,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:80
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_http_Custom_Local_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_https_Standard_Local_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "https",
		LocalHost:      "localhost",
		LocalPort:      8443,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy https://localhost:8443
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_https_Custom_Local_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "https",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy https://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

// layer 7, public address variations

func TestPublicService_AsCaddyConfigEntry_Layer7_With_https_Standard_Public_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "https",
		PublicHost:     "example.com",
		PublicPort:     443,
		Params:         []PublicServiceParam{},
	}

	expected := `
https://example.com {
    reverse_proxy http://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_https_Custom_Public_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "https",
		PublicHost:     "example.com",
		PublicPort:     8443,
		Params:         []PublicServiceParam{},
	}

	expected := `
example.com:8443 {
    reverse_proxy http://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_http_Standard_Public_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     80,
		Params:         []PublicServiceParam{},
	}

	expected := `
http://example.com {
    reverse_proxy http://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer7_With_http_Custom_Public_Port(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "http",
		LocalHost:      "localhost",
		LocalPort:      8080,
		PublicProtocol: "http",
		PublicHost:     "example.com",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expected := `
example.com:8080 {
    reverse_proxy http://localhost:8080
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

// layer 4, local address variations

func TestPublicService_AsCaddyConfigEntry_Layer4_With_No_BlockParams(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "tcp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "tcp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expected := `
127.0.0.1:8080 {
    route {
        proxy {
            upstream tcp/192.168.1.100:8080
        }
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_1_BlockParam(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "tcp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "tcp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{{ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "dial_timeout 5s"}},
	}

	expected := `
127.0.0.1:8080 {
    route {
        proxy {
            upstream tcp/192.168.1.100:8080 {
                dial_timeout 5s
            }
        }
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_Multiple_BlockParams(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "tcp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "tcp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{{ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "dial_timeout 5s"}, {ParamType: PublicServiceParamTypeCaddyFreeText, ParamValue: "keepalive_interval 30s"}},
	}

	expected := `
127.0.0.1:8080 {
    route {
        proxy {
            upstream tcp/192.168.1.100:8080 {
                dial_timeout 5s
                keepalive_interval 30s
            }
        }
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_udp_to_udp(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "udp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "udp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expected := `
127.0.0.1:8080 {
    route {
        proxy {
            upstream udp/192.168.1.100:8080
        }
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_tcp_to_tcp(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "tcp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "tcp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expected := `
127.0.0.1:8080 {
    route {
        proxy {
            upstream tcp/192.168.1.100:8080
        }
    }
}
`

	got, err := service.AsCaddyConfigEntry()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expected) {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_tcp_to_udp(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "udp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "tcp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expectedResult := ""
	expectedError := "for layer 4, local protocol and public protocol must be the same (udp -> udp or tcp -> tcp)"

	got, err := service.AsCaddyConfigEntry()

	if err == nil {
		t.Errorf("expected error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expectedResult) {
		t.Errorf("expected empty string, got %s", got)
	}

	if err.Error() != expectedError {
		t.Errorf("expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestPublicService_AsCaddyConfigEntry_Layer4_With_udp_to_tcp(t *testing.T) {
	service := PublicService{
		LocalProtocol:  "tcp",
		LocalHost:      "192.168.1.100",
		LocalPort:      8080,
		PublicProtocol: "udp",
		PublicHost:     "127.0.0.1",
		PublicPort:     8080,
		Params:         []PublicServiceParam{},
	}

	expectedResult := ""
	expectedError := "for layer 4, local protocol and public protocol must be the same (udp -> udp or tcp -> tcp)"

	got, err := service.AsCaddyConfigEntry()

	if err == nil {
		t.Errorf("expected error, got %v", err)
	}

	if removeSpaces(got) != removeSpaces(expectedResult) {
		t.Errorf("expected empty string, got %s", got)
	}

	if err.Error() != expectedError {
		t.Errorf("expected error '%s', got '%s'", expectedError, err.Error())
	}
}
