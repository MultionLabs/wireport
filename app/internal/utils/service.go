package utils

import (
	"fmt"
	"io"
	"net/http"
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
