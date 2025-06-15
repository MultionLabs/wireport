package types

import "wireport/internal/nodes/types"

type JoinRequestDTO struct {
	JoinToken string `json:"joinToken"`
}

type JoinResponseDTO struct {
	NodeConfig *types.Node `json:"node"`
}
