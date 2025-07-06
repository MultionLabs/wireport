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
)

type APIService struct {
	client           *http.Client
	clientCertBundle *mtls.FullClientBundle
}

func NewAPIService(clientCertBundle *mtls.FullClientBundle) *APIService {
	var (
		dnsResolverTimeout = time.Duration(10 * time.Second)
	)

	// Create a dialer with timeout and fallback DNS resolution
	dialer := &net.Dialer{
		Timeout:   10 * time.Second, // Connection timeout
		KeepAlive: 10 * time.Second, // Keep-alive timeout
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Try multiple DNS servers for better reliability
				dnsServers := []string{
					"8.8.8.8:53",        // Google DNS
					"1.1.1.1:53",        // Cloudflare DNS
					"208.67.222.222:53", // OpenDNS
				}

				for _, dnsServer := range dnsServers {
					d := net.Dialer{
						Timeout: dnsResolverTimeout,
					}

					conn, err := d.DialContext(ctx, "udp", dnsServer)
					if err == nil {
						return conn, nil
					}

				}

				// Fallback to system default DNS
				d := net.Dialer{
					Timeout: dnsResolverTimeout,
				}
				return d.DialContext(ctx, network, address)
			},
		},
	}

	tlsConfig, err := clientCertBundle.GetClientTLSConfig()

	if err != nil {
		panic(err)
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSClientConfig:       tlsConfig,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &APIService{
		client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second, // Overall request timeout
		},
		clientCertBundle: clientCertBundle,
	}
}

func (s *APIService) Join(joinToken string, gatewayAddress string) (*types.JoinResponseDTO, error) {
	payload := types.JoinRequestDTO{
		JoinToken: joinToken,
	}

	url := fmt.Sprintf("https://%s/commands/join", gatewayAddress)

	const (
		maxRetries = 5
		baseDelay  = 1 * time.Second  // start with 1s backoff
		maxDelay   = 10 * time.Second // cap individual wait at 10s
	)

	firstAttemptAt := time.Now()
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

	return nil, fmt.Errorf("failed to join after %d attempts, first attempt at %v, now %v, last error: %w", maxRetries, firstAttemptAt, time.Now(), lastErr)
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

	// Also treat DNS errors (e.g., "no such host") as retryable because they may be transient
	var dnsErr *net.DNSError
	if ok := errors.As(err, &dnsErr); ok {
		return true
	}

	return false
}
