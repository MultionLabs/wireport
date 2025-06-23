package join_requests

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
	encryption_aes "wireport/internal/encryption/aes"
	join_requests_types "wireport/internal/join-requests/types"
)

type APIService struct {
	client *http.Client
}

func NewAPIService() *APIService {
	var (
		dnsResolverAddress = "8.8.8.8:53"
		dnsResolverProto   = "udp"
		dnsResolverTimeout = time.Duration(5 * time.Second)
	)

	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: dnsResolverTimeout,
				}

				return d.DialContext(ctx, dnsResolverProto, dnsResolverAddress)
			},
		},
	}

	transport := &http.Transport{
		DialContext: dialer.DialContext,
	}

	return &APIService{
		client: &http.Client{
			Transport: transport,
		},
	}
}

func (s *APIService) Join(joinToken string) (*join_requests_types.JoinResponseDTO, error) {
	joinRequest := &join_requests_types.JoinRequest{}

	err := joinRequest.FromBase64(joinToken)

	if err != nil {
		return nil, ErrFailedToParseJoinToken
	}

	payload := join_requests_types.JoinRequestDTO{
		JoinToken: joinToken,
	}

	joinResponse, err := encryption_aes.EncryptedAPIRequest[join_requests_types.JoinResponseDTO](s.client, fmt.Sprintf("http://%s/join", joinRequest.HostAddress), payload, joinRequest.Id, joinRequest.EncryptionKeyBase64)

	if err != nil {
		return nil, ErrFailedToSendJoinRequest
	}

	return joinResponse, nil
}

func (s *APIService) GetPublicIP() (*string, error) {
	resp, err := s.client.Get("https://ipinfo.io/ip")

	if err != nil {
		return nil, ErrFailedToGetPublicIP
	}

	defer resp.Body.Close()

	publicIP, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, ErrFailedToReadPublicIP
	}

	publicIPString := string(publicIP)

	return &publicIPString, nil
}
