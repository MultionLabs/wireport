package terminal

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

func RestartServices(restartWireguard bool, restartCoreDNS bool, restartCaddy bool) error {
	logger.Info("Restarting services: Wireguard: %v, CoreDNS: %v, Caddy: %v", restartWireguard, restartCoreDNS, restartCaddy)

	if restartWireguard {
		if _, err := os.Stat(config.Config.WireguardConfigPath); !os.IsNotExist(err) {
			err := exec.Command("/bin/sh", "-c", config.Config.WireguardRestartCommand).Run()

			if err != nil {
				return fmt.Errorf("failed to restart wireguard: %v", err)
			} else {
				logger.Info("Wireguard restarted")
			}
		}
	}

	if restartCoreDNS {
		if _, err := os.Stat(config.Config.CoreDNSConfigPath); !os.IsNotExist(err) {
			err = exec.Command("/bin/sh", "-c", config.Config.CoreDNSRestartCommand).Run()

			if err != nil {
				return fmt.Errorf("failed to restart coredns: %v", err)
			} else {
				logger.Info("CoreDNS restarted")
			}
		}
	}

	if restartCaddy {
		if _, err := os.Stat(config.Config.CaddyConfigPath); !os.IsNotExist(err) {
			err = exec.Command("/bin/sh", "-c", fmt.Sprintf(config.Config.CaddyRestartCommand, config.Config.CaddyConfigPath)).Run()

			if err != nil {
				return fmt.Errorf("failed to restart caddy: %v", err)
			} else {
				logger.Info("Caddy restarted")
			}
		}
	}

	return nil
}

var (
	restartChan  = make(chan struct{}, 1)
	restartOnce  sync.Once
	restartMutex sync.Mutex
	isRestarting atomic.Bool
)

// ScheduleRestart schedules a service restart to happen after a delay
func ScheduleRestart(delay time.Duration, restartWireguard bool, restartCoreDNS bool, restartCaddy bool) {
	restartOnce.Do(func() {
		go func() {
			for range restartChan {
				// Use mutex to ensure only one restart process runs at a time
				restartMutex.Lock()
				if isRestarting.Load() {
					restartMutex.Unlock()
					continue
				}
				isRestarting.Store(true)
				restartMutex.Unlock()

				time.Sleep(delay)
				if err := RestartServices(restartWireguard, restartCoreDNS, restartCaddy); err != nil {
					logger.Error("Failed to restart services: %v", err)
				}

				isRestarting.Store(false)
			}
		}()
	})

	// Non-blocking send to channel
	select {
	case restartChan <- struct{}{}:
		logger.Info("Service restart scheduled in %v", delay)
	default:
		// If there's already a pending restart, we don't need to schedule another one
		logger.Info("Service restart already scheduled")
	}
}
