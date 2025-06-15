package public_services

import "gorm.io/gorm"

type PublicServiceRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *PublicServiceRepository {
	return &PublicServiceRepository{db: db}
}

func (r *PublicServiceRepository) Save(service *PublicService) error {
	return r.db.Save(service).Error
}

func (r *PublicServiceRepository) GetAll() []*PublicService {
	var services []*PublicService

	if err := r.db.Find(&services).Error; err != nil {
		return nil
	}

	return services
}

func (r *PublicServiceRepository) Delete(publicProtocol, publicHost string, publicPort uint16) bool {
	result := r.db.Delete(&PublicService{}, "public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort)

	if result.Error != nil {
		return false
	}

	return result.RowsAffected > 0
}
