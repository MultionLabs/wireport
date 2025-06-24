package join_requests

import (
	"encoding/base64"
	"time"
	encryption_aes "wireport/internal/encryption/aes"
	"wireport/internal/encryption/mtls"
	"wireport/internal/join-requests/types"
	nodeTypes "wireport/internal/nodes/types"

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

func (r *Repository) Create(id string, hostAddress nodeTypes.UDPAddrMarshable, dockerSubnet *string, role nodeTypes.NodeRole, clientCertBundle *mtls.FullClientBundle) (*types.JoinRequest, error) {
	encryptionKey, err := encryption_aes.GenerateAESKey()

	if err != nil {
		return nil, err
	}

	encryptionKeyBase64 := base64.StdEncoding.EncodeToString(encryptionKey)

	request := &types.JoinRequest{
		Id:                  id,
		EncryptionKeyBase64: encryptionKeyBase64,
		ClientCertBundle:    *clientCertBundle,
		HostAddress:         hostAddress.String(),
		Role:                role,
		CreatedAt:           time.Now(),
		DockerSubnet:        dockerSubnet,
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

func (r *Repository) CountAll() int {
	var count int64

	if err := r.db.Model(&types.JoinRequest{}).Count(&count).Error; err != nil {
		return 0
	}

	return int(count)
}

func (r *Repository) CountServerJoinRequests() int {
	// client & server nodes use docker subnets
	var count int64

	if err := r.db.Model(&types.JoinRequest{}).Where("role = ?", nodeTypes.NodeRoleServer).Count(&count).Error; err != nil {
		return 0
	}

	return int(count)
}
