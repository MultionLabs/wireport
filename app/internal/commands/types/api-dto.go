package types

import (
	node_types "wireport/internal/nodes/types"
	"wireport/internal/publicservices"
)

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

type ServerRemoveRequestDTO struct {
	NodeID string `json:"nodeID"`
}

type ClientNewRequestDTO struct {
	JoinRequest bool `json:"joinRequest"`
	Quiet       bool `json:"quiet"`
	Wait        bool `json:"wait"`
}

type ClientListRequestDTO struct {
}

type ServerListRequestDTO struct {
}

type ServicePublishRequestDTO struct {
	LocalProtocol  string `json:"localProtocol"`
	LocalHost      string `json:"localHost"`
	LocalPort      uint16 `json:"localPort"`
	PublicProtocol string `json:"publicProtocol"`
	PublicHost     string `json:"publicHost"`
	PublicPort     uint16 `json:"publicPort"`
}

type ServiceUnpublishRequestDTO struct {
	PublicProtocol string `json:"publicProtocol"`
	PublicHost     string `json:"publicHost"`
	PublicPort     uint16 `json:"publicPort"`
}

type ServiceParamNewRequestDTO struct {
	PublicProtocol string                                `json:"publicProtocol"`
	PublicHost     string                                `json:"publicHost"`
	PublicPort     uint16                                `json:"publicPort"`
	ParamType      publicservices.PublicServiceParamType `json:"paramType"`
	ParamValue     string                                `json:"paramValue"`
}

type ServiceParamRemoveRequestDTO struct {
	PublicProtocol string                                `json:"publicProtocol"`
	PublicHost     string                                `json:"publicHost"`
	PublicPort     uint16                                `json:"publicPort"`
	ParamType      publicservices.PublicServiceParamType `json:"paramType"`
	ParamValue     string                                `json:"paramValue"`
}

type ServiceParamListRequestDTO struct {
	PublicProtocol string `json:"publicProtocol"`
	PublicHost     string `json:"publicHost"`
	PublicPort     uint16 `json:"publicPort"`
}

type ServiceListRequestDTO struct {
}

// join requests

type JoinRequestDTO struct {
	JoinToken string `json:"joinToken"`
}

type JoinResponseDTO struct {
	NodeConfig *node_types.Node `json:"node"`
}
