package network_apps

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
	"wireport/cmd/server/config"
	"wireport/internal/logger"
)

func RestartNetworkApps(restartWireguard bool, restartCoreDNS bool, restartCaddy bool) error {
	logger.Info("Restarting network apps: Wireguard: %v, CoreDNS: %v, Caddy: %v", restartWireguard, restartCoreDNS, restartCaddy)

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
	networkAppsRestartChan  = make(chan struct{}, 1)
	networkAppsRestartOnce  sync.Once
	networkAppsRestartMutex sync.Mutex
	isNetworkAppsRestarting atomic.Bool
)

// Schedule a network apps restart to happen after a delay
func ScheduleNetworkAppsRestart(delay time.Duration, restartWireguard bool, restartCoreDNS bool, restartCaddy bool) {
	networkAppsRestartOnce.Do(func() {
		go func() {
			for range networkAppsRestartChan {
				// Use mutex to ensure only one restart process runs at a time
				networkAppsRestartMutex.Lock()
				if isNetworkAppsRestarting.Load() {
					networkAppsRestartMutex.Unlock()
					continue
				}
				isNetworkAppsRestarting.Store(true)
				networkAppsRestartMutex.Unlock()

				time.Sleep(delay)

				if err := RestartNetworkApps(restartWireguard, restartCoreDNS, restartCaddy); err != nil {
					logger.Error("Failed to restart network apps: %v", err)
				}

				isNetworkAppsRestarting.Store(false)
			}
		}()
	})

	// Non-blocking send to channel
	select {
	case networkAppsRestartChan <- struct{}{}:
		logger.Info("Network apps restart scheduled in %v", delay)
	default:
		// If there's already a pending restart, we don't need to schedule another one
		logger.Info("Network apps restart already scheduled")
	}
}
