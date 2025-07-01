package types

import (
	"encoding/base64"
	"encoding/json"
	"time"
	"wireport/internal/encryption/mtls"
	nodeTypes "wireport/internal/nodes/types"
)

type JoinRequest struct {
	ID                  string                `gorm:"type:text;primary_key" json:"id"`
	EncryptionKeyBase64 string                `gorm:"type:text" json:"key"`
	ClientCertBundle    mtls.FullClientBundle `gorm:"type:text;serializer:json" json:"clientCertBundle"`
	DockerSubnet        *string               `gorm:"type:text" json:"dockerSubnet"`
	GatewayAddress      string                `gorm:"type:text" json:"gateway"`
	Role                nodeTypes.NodeRole    `gorm:"type:text" json:"role"`

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
