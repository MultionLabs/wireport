package terminal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"wireport/cmd/server/config"
	"wireport/internal/logger"
)

type Command struct {
	Command string
	Args    []string
	Dir     string
}

func NewCommand(command string, args ...string) *Command {
	return &Command{
		Command: command,
		Args:    args,
	}
}

func (c *Command) Execute() (string, error) {
	cmd := exec.Command(c.Command, c.Args...)

	if c.Dir != "" {
		cmd.Dir = c.Dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %v\nStderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

func RestartServices() error {
	logger.Info("Restarting services")

	if _, err := os.Stat(config.Config.WireguardConfigPath); !os.IsNotExist(err) {
		err := exec.Command("/bin/sh", "-c", config.Config.WireguardRestartCommand).Run()

		if err != nil {
			return fmt.Errorf("failed to restart wireguard: %v", err)
		} else {
			logger.Info("Wireguard restarted")
		}
	}

	if _, err := os.Stat(config.Config.DNSMasqConfigPath); !os.IsNotExist(err) {
		err = exec.Command("/bin/sh", "-c", config.Config.DNSMasqRestartCommand).Run()

		if err != nil {
			return fmt.Errorf("failed to restart dnsmasq: %v", err)
		} else {
			logger.Info("DNSMasq restarted")
		}
	}

	if _, err := os.Stat(config.Config.CaddyConfigPath); !os.IsNotExist(err) {
		err = exec.Command("/bin/sh", "-c", fmt.Sprintf(config.Config.CaddyRestartCommand, config.Config.CaddyConfigPath)).Run()

		if err != nil {
			return fmt.Errorf("failed to restart caddy: %v", err)
		} else {
			logger.Info("Caddy restarted")
		}
	}

	return nil
}
