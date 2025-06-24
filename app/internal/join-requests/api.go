package join_requests

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
	encryption_aes "wireport/internal/encryption/aes"
	"wireport/internal/encryption/mtls"
	join_requests_types "wireport/internal/join-requests/types"
)

type APIService struct {
	client           *http.Client
	clientCertBundle *mtls.FullClientBundle
}

func NewAPIService(clientCertBundle *mtls.FullClientBundle) *APIService {
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

	tlsConfig, err := clientCertBundle.GetClientTLSConfig()

	if err != nil {
		panic(err)
	}

	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}

	return &APIService{
		client: &http.Client{
			Transport: transport,
		},
		clientCertBundle: clientCertBundle,
	}
}

func (s *APIService) Join(joinToken string, joinRequest *join_requests_types.JoinRequest) (*join_requests_types.JoinResponseDTO, error) {
	payload := join_requests_types.JoinRequestDTO{
		JoinToken: joinToken,
	}

	joinResponse, err := encryption_aes.EncryptedAPIRequest[join_requests_types.JoinResponseDTO](s.client, fmt.Sprintf("https://%s/join", joinRequest.HostAddress), payload, joinRequest.Id, joinRequest.EncryptionKeyBase64)

	if err != nil {
		return nil, ErrFailedToSendJoinRequest
	}

	return joinResponse, nil
}
