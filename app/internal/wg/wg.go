package wg

import (
	"wireport/internal/terminal"
)

func GenerateKeyPair() (string, string, error) {
	privateKey, err := terminal.NewCommand("wg", "genkey").Execute()

	if err != nil {
		return "", "", err
	}

	publicKey, err := terminal.NewCommand("sh", "-c", "echo "+privateKey+" | wg pubkey").Execute()

	if err != nil {
		return "", "", err
	}

	return privateKey, publicKey, nil
}
