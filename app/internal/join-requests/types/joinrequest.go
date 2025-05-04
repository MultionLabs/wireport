package types

import (
	"encoding/base64"
	"encoding/json"
	"time"
	nodeTypes "wireport/internal/nodes/types"
)

type JoinRequest struct {
	Id                  string             `json:"id"`
	EncryptionKeyBase64 string             `json:"key"`
	HostAddress         string             `json:"host"`
	Role                nodeTypes.NodeRole `json:"role"`

	CreatedAt time.Time
}

func (c *JoinRequest) ToBase64() (*string, error) {
	val, err := json.Marshal(c)

	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(val)

	return &encoded, nil
}

func (c *JoinRequest) FromBase64(base64String string) error {
	decoded, err := base64.StdEncoding.DecodeString(base64String)

	if err != nil {
		return err
	}

	err = json.Unmarshal(decoded, c)

	if err != nil {
		return err
	}

	return nil
}
