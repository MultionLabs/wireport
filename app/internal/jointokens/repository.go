package jointokens

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(token string) (*JoinToken, error) {
	joinToken := &JoinToken{
		Token:     token,
		CreatedAt: time.Now(),
	}

	err := r.db.Create(joinToken).Error

	if err != nil {
		return nil, err
	}

	return joinToken, nil
}

func (r *Repository) GetLast() (*JoinToken, error) {
	joinToken := &JoinToken{}

	err := r.db.Order("created_at DESC").First(joinToken).Error

	if err != nil {
		return nil, err
	}

	return joinToken, nil
}

func (r *Repository) DeleteAll() error {
	return r.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&JoinToken{}).Error
}
