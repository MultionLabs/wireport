package types

import node_types "wireport/internal/nodes/types"

type ExecRequestDTO struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type ExecResponseDTO struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

type ServerNewRequestDTO struct {
	Force        bool   `json:"force"`
	Quiet        bool   `json:"quiet"`
	DockerSubnet string `json:"dockerSubnet"`
}

type ClientNewRequestDTO struct {
	JoinRequest bool `json:"joinRequest"`
	Quiet       bool `json:"quiet"`
	Wait        bool `json:"wait"`
}

// join requests

type JoinRequestDTO struct {
	JoinToken string `json:"joinToken"`
}

type JoinResponseDTO struct {
	NodeConfig *node_types.Node `json:"node"`
}
