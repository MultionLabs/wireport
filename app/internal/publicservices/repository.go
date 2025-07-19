package publicservices

import (
	"errors"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(service *PublicService) error {
	return r.db.Save(service).Error
}

func (r *Repository) GetAll() ([]*PublicService, error) {
	var services []*PublicService

	if err := r.db.Find(&services).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return services, nil
		}

		return nil, err
	}

	return services, nil
}

func (r *Repository) Delete(publicProtocol, publicHost string, publicPort uint16) bool {
	result := r.db.Delete(&PublicService{}, "public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort)

	if result.Error != nil {
		return false
	}

	return result.RowsAffected > 0
}

func (r *Repository) Get(publicProtocol, publicHost string, publicPort uint16) (result *PublicService, err error) {
	var service PublicService

	err = r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}

	return &service, nil
}

func (r *Repository) AddParam(publicProtocol, publicHost string, publicPort uint16, paramType PublicServiceParamType, paramValue string) bool {
	var service PublicService

	err := r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error

	if err != nil {
		return false
	}

	for _, p := range service.Params {
		if p.ParamType == paramType && p.ParamValue == paramValue {
			// param already exists
			return false
		}
	}

	service.Params = append(service.Params, PublicServiceParam{ParamType: paramType, ParamValue: paramValue})

	result := r.db.Save(&service)

	return result.Error == nil && result.RowsAffected > 0
}

func (r *Repository) RemoveParam(publicProtocol, publicHost string, publicPort uint16, paramType PublicServiceParamType, paramValue string) bool {
	var service PublicService

	err := r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error

	if err != nil {
		return false
	}

	newParams := []PublicServiceParam{}
	paramFound := false

	for _, p := range service.Params {
		if p.ParamType == paramType && p.ParamValue == paramValue {
			paramFound = true
			continue
		}

		newParams = append(newParams, p)
	}

	if !paramFound {
		// param not found - nothing to remove
		return false
	}

	service.Params = newParams

	result := r.db.Save(&service)

	return result.Error == nil && result.RowsAffected > 0
}
