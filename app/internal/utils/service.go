package utils

import (
	"fmt"
	"io"
	"net/http"
)

func GetPublicIP() (*string, error) {
	resp, err := http.Get("https://ipinfo.io/ip")

	if err != nil {
		return nil, fmt.Errorf("failed to get public ip: %w", err)
	}

	defer resp.Body.Close()

	publicIP, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read public ip: %w", err)
	}

	publicIPString := string(publicIP)

	return &publicIPString, nil
}
