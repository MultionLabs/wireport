package dockersocket

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/aymerick/raymond"

	"wireport/cmd/server/config"
	"wireport/internal/nodes"
	templates "wireport/internal/templates"
)

/*
* Publishing/unpublishing docker socket comes down to enabling/disabling the runit service by moving it from the disabled directory to the service directory and vice versa.
 */

func servicePath() string {
	return filepath.Join(config.Config.RunitServiceDir, config.Config.DockerSocketServiceName)
}

func disabledPath() string {
	return filepath.Join(config.Config.RunitServiceDisabledDir, config.Config.DockerSocketServiceName)
}

func isServiceEnabled() bool {
	_, err := os.Stat(servicePath())
	return err == nil
}

func isSocatRunning() bool {
	return exec.Command("pgrep", "-x", "socat").Run() == nil
}

func templateVars() map[string]string {
	return map[string]string{
		"dockerSocketServicePath":  servicePath(),
		"dockerSocketDisabledPath": disabledPath(),
		"dockerSocketServiceName":  config.Config.DockerSocketServiceName,
		"dockerSocketTCPPort":      config.Config.DockerSocketTCPPort,
		"runitServiceDir":          config.Config.RunitServiceDir,
		"runitServiceDisabledDir":  config.Config.RunitServiceDisabledDir,
	}
}

func renderScript(templatePath string) (string, error) {
	templateBytes, err := templates.Scripts.ReadFile(templatePath)
	if err != nil {
		return "", err
	}

	tpl, err := raymond.Parse(string(templateBytes))
	if err != nil {
		return "", err
	}

	return tpl.Exec(templateVars())
}

func runScript(script string) (string, error) {
	cmd := exec.Command("/bin/sh", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}

	return string(output), nil
}

func publish() error {
	script, err := renderScript(config.Config.PublishDockerSocketScriptTemplatePath)
	if err != nil {
		return err
	}

	_, err = runScript(script)
	return err
}

func unpublish() error {
	script, err := renderScript(config.Config.UnpublishDockerSocketScriptTemplatePath)
	if err != nil {
		return err
	}

	_, err = runScript(script)
	return err
}

// ReconcileWithLabels ensures the socat-docker-socket runit service matches the node labels.
func ReconcileWithLabels(labels []string, stdOut, errOut io.Writer) {
	wantPublished := slices.Contains(labels, nodes.DockerSocketPublishedLabel)
	enabled := isServiceEnabled()
	running := isSocatRunning()

	if wantPublished {
		if enabled && running {
			return
		}

		fmt.Fprintf(stdOut, "Enabling docker socket TCP gateway (label %q present)\n", nodes.DockerSocketPublishedLabel)

		if err := publish(); err != nil {
			fmt.Fprintf(errOut, "Failed to enable docker socket TCP gateway: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "Docker socket TCP gateway enabled\n")
		return
	}

	if !enabled && !running {
		return
	}

	fmt.Fprintf(stdOut, "Disabling docker socket TCP gateway (label %q absent)\n", nodes.DockerSocketPublishedLabel)

	if err := unpublish(); err != nil {
		fmt.Fprintf(errOut, "Failed to disable docker socket TCP gateway: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "Docker socket TCP gateway disabled\n")
}
