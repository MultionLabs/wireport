package join_requests

import (
	"encoding/base64"
	"time"
	"wireport/internal/encryption"
	"wireport/internal/join-requests/types"
	nodeTypes "wireport/internal/nodes/types"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Create(hostAddress nodeTypes.UDPAddrMarshable) (*types.JoinRequest, error) {
	encryptionKey, err := encryption.GenerateKey()

	if err != nil {
		return nil, err
	}

	encryptionKeyBase64 := base64.StdEncoding.EncodeToString(encryptionKey)

	request := &types.JoinRequest{
		Id:                  uuid.New().String(),
		EncryptionKeyBase64: encryptionKeyBase64,
		HostAddress:         hostAddress.String(),
		Role:                nodeTypes.NodeRoleServer,
		CreatedAt:           time.Now(),
	}

	err = r.db.Create(request).Error

	if err != nil {
		return nil, err
	}

	return request, nil
}

func (r *Repository) Get(id string) (*types.JoinRequest, error) {
	var request types.JoinRequest

	if err := r.db.First(&request, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &request, nil
}

func (r *Repository) Delete(id string) error {
	return r.db.Delete(&types.JoinRequest{}, "id = ?", id).Error
}
