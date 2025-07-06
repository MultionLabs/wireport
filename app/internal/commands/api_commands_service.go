package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
	"wireport/internal/commands/types"
	"wireport/internal/encryption/mtls"
	"wireport/internal/logger"
	"wireport/internal/publicservices"
)

type APICommandsService struct {
	Host             string
	Port             uint16
	ClientCertBundle *mtls.FullClientBundle
}

func makeSecureRequestWithResponse[RequestType any, ResponseType any](api *APICommandsService, method, endpoint string, request RequestType) (ResponseType, error) {
	var response ResponseType

	requestBody, err := json.Marshal(request)
	if err != nil {
		return response, fmt.Errorf("failed to marshal request body: %v", err)
	}

	tlsConfig, err := api.ClientCertBundle.GetClientTLSConfig()
	if err != nil {
		return response, fmt.Errorf("failed to get client TLS config: %v", err)
	}

	url := fmt.Sprintf("https://%s:%d%s", api.Host, api.Port, endpoint)
	httpRequest, err := http.NewRequest(method, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return response, fmt.Errorf("failed to create request: %v", err)
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	// Create a dialer with timeout
	dialer := &net.Dialer{
		Timeout:   30 * time.Second, // Connection timeout
		KeepAlive: 30 * time.Second, // Keep-alive timeout
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second, // Overall request timeout
	}

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return response, fmt.Errorf("failed to execute request: %v", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return response, fmt.Errorf("failed to read response body: %v", err)
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return response, fmt.Errorf("failed to unmarshal response body: %v", err)
	}

	return response, nil
}

func (a *APICommandsService) ServerNew(force bool, quiet bool, dockerSubnet string) (types.ExecResponseDTO, error) {
	serverNewResponseDTO, err := makeSecureRequestWithResponse[types.ServerNewRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/server/new",
		types.ServerNewRequestDTO{
			Force:        force,
			Quiet:        quiet,
			DockerSubnet: dockerSubnet,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serverNewResponseDTO, nil
}

func (a *APICommandsService) ServerRemove(nodeID string) (types.ExecResponseDTO, error) {
	serverRemoveResponseDTO, err := makeSecureRequestWithResponse[types.ServerRemoveRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/server/remove",
		types.ServerRemoveRequestDTO{
			NodeID: nodeID,
		},
	)

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serverRemoveResponseDTO, nil
}

func (a *APICommandsService) ClientNew(force bool, quiet bool, wait bool) (types.ExecResponseDTO, error) {
	clientNewResponseDTO, err := makeSecureRequestWithResponse[types.ClientNewRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/client/new",
		types.ClientNewRequestDTO{
			JoinRequest: force,
			Quiet:       quiet,
			Wait:        wait,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return clientNewResponseDTO, nil
}

func (a *APICommandsService) ClientList() (types.ExecResponseDTO, error) {
	clientListResponseDTO, err := makeSecureRequestWithResponse[types.ClientListRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/client/list",
		types.ClientListRequestDTO{},
	)

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return clientListResponseDTO, nil
}

func (a *APICommandsService) ServerList() (types.ServerListResponseDTO, error) {
	serverListResponseDTO, err := makeSecureRequestWithResponse[types.ServerListRequestDTO, types.ServerListResponseDTO](
		a, "POST", "/commands/server/list",
		types.ServerListRequestDTO{},
	)

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ServerListResponseDTO{}, err
	}

	return serverListResponseDTO, nil
}

func (a *APICommandsService) ServicePublish(localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) (types.ExecResponseDTO, error) {
	servicePublishResponseDTO, err := makeSecureRequestWithResponse[types.ServicePublishRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/publish",
		types.ServicePublishRequestDTO{
			LocalProtocol:  localProtocol,
			LocalHost:      localHost,
			LocalPort:      localPort,
			PublicProtocol: publicProtocol,
			PublicHost:     publicHost,
			PublicPort:     publicPort,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return servicePublishResponseDTO, nil
}

func (a *APICommandsService) ServiceUnpublish(publicProtocol string, publicHost string, publicPort uint16) (types.ExecResponseDTO, error) {
	serviceUnpublishResponseDTO, err := makeSecureRequestWithResponse[types.ServiceUnpublishRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/unpublish",
		types.ServiceUnpublishRequestDTO{
			PublicProtocol: publicProtocol,
			PublicHost:     publicHost,
			PublicPort:     publicPort,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serviceUnpublishResponseDTO, nil
}

func (a *APICommandsService) ServiceList() (types.ExecResponseDTO, error) {
	serviceListResponseDTO, err := makeSecureRequestWithResponse[types.ServiceListRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/list",
		types.ServiceListRequestDTO{},
	)

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serviceListResponseDTO, nil
}

func (a *APICommandsService) ServiceParamNew(publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) (types.ExecResponseDTO, error) {
	serviceParamNewResponseDTO, err := makeSecureRequestWithResponse[types.ServiceParamNewRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/params/new",
		types.ServiceParamNewRequestDTO{
			PublicProtocol: publicProtocol,
			PublicHost:     publicHost,
			PublicPort:     publicPort,
			ParamType:      paramType,
			ParamValue:     paramValue,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serviceParamNewResponseDTO, nil
}

func (a *APICommandsService) ServiceParamRemove(publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) (types.ExecResponseDTO, error) {
	serviceParamRemoveResponseDTO, err := makeSecureRequestWithResponse[types.ServiceParamRemoveRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/params/remove",
		types.ServiceParamRemoveRequestDTO{
			PublicProtocol: publicProtocol,
			PublicHost:     publicHost,
			PublicPort:     publicPort,
			ParamType:      paramType,
			ParamValue:     paramValue,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serviceParamRemoveResponseDTO, nil
}

func (a *APICommandsService) ServiceParamList(publicProtocol string, publicHost string, publicPort uint16) (types.ExecResponseDTO, error) {
	serviceParamListResponseDTO, err := makeSecureRequestWithResponse[types.ServiceParamListRequestDTO, types.ExecResponseDTO](
		a, "POST", "/commands/service/params/list",
		types.ServiceParamListRequestDTO{
			PublicProtocol: publicProtocol,
			PublicHost:     publicHost,
			PublicPort:     publicPort,
		})

	if err != nil {
		logger.Error("Failed to marshal request body: %v", err)
		return types.ExecResponseDTO{}, err
	}

	return serviceParamListResponseDTO, nil
}
