package joinrequests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
	"wireport/internal/commands/types"
	"wireport/internal/encryption/mtls"
	joinrequeststypes "wireport/internal/joinrequests/types"
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
			Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
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

func (s *APIService) Join(joinToken string, joinRequest *joinrequeststypes.JoinRequest) (*types.JoinResponseDTO, error) {
	payload := types.JoinRequestDTO{
		JoinToken: joinToken,
	}

	url := fmt.Sprintf("https://%s/commands/join", joinRequest.HostAddress)

	const (
		maxRetries = 5
		baseDelay  = 1 * time.Second
		maxDelay   = 10 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		requestPayloadJSON, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}

		resp, err := s.client.Post(
			url,
			"application/json",
			bytes.NewBuffer(requestPayloadJSON),
		)

		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			if !isRetryableError(err) {
				break
			}
			if attempt == maxRetries {
				break
			}
			delay := calculateBackoffDelay(attempt, baseDelay, maxDelay)
			time.Sleep(delay)
			continue
		}

		defer resp.Body.Close()

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)
			if !isRetryableError(err) {
				break
			}
			if attempt == maxRetries {
				break
			}
			delay := calculateBackoffDelay(attempt, baseDelay, maxDelay)
			time.Sleep(delay)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(responseBody))
			if attempt == maxRetries {
				break
			}
			delay := calculateBackoffDelay(attempt, baseDelay, maxDelay)
			time.Sleep(delay)
			continue
		}

		var joinResponse types.JoinResponseDTO
		err = json.Unmarshal(responseBody, &joinResponse)
		if err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			if attempt == maxRetries {
				break
			}
			delay := calculateBackoffDelay(attempt, baseDelay, maxDelay)
			time.Sleep(delay)
			continue
		}

		return &joinResponse, nil
	}

	return nil, fmt.Errorf("failed to join after %d attempts, last error: %w", maxRetries, lastErr)
}

func calculateBackoffDelay(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	delay := baseDelay * time.Duration(1<<(attempt-1))
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.ECONNREFUSED, // Connection refused
			syscall.ENETUNREACH,  // Network is unreachable
			syscall.EHOSTUNREACH, // No route to host
			syscall.ETIMEDOUT:    // Connection timed out
			return true
		}
	}

	// Also check for context deadline exceeded (timeout)
	var deadlineErr interface{ Timeout() bool }
	if ok := errors.As(err, &deadlineErr); ok && deadlineErr.Timeout() {
		return true
	}

	return false
}
