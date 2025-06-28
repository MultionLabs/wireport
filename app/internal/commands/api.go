package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"wireport/internal/commands/types"
	"wireport/internal/encryption/mtls"
	"wireport/internal/logger"
)

type APIService struct {
	Host             string
	Port             uint16
	ClientCertBundle *mtls.FullClientBundle
}

func makeSecureRequestWithResponse[RequestType any, ResponseType any](api *APIService, method, endpoint string, request RequestType) (ResponseType, error) {
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

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
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

func (a *APIService) ServerNew(force bool, quiet bool, dockerSubnet string) (types.ExecResponseDTO, error) {
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

func (a *APIService) ClientNew(force bool, quiet bool, wait bool) (types.ExecResponseDTO, error) {
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
