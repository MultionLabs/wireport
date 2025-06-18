package terminal

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
