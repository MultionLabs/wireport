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

func (r *PublicServiceRepository) Get(publicProtocol, publicHost string, publicPort uint16) (result *PublicService, err error) {
	var service PublicService

	if err := r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error; err != nil {
		return nil, err
	}

	return &service, nil
}

func (r *PublicServiceRepository) AddParam(publicProtocol, publicHost string, publicPort uint16, paramType PublicServiceParamType, paramValue string) bool {
	var service PublicService

	if err := r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error; err != nil {
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

func (r *PublicServiceRepository) RemoveParam(publicProtocol, publicHost string, publicPort uint16, paramType PublicServiceParamType, paramValue string) bool {
	var service PublicService

	if err := r.db.Where("public_protocol = ? AND public_host = ? AND public_port = ?", publicProtocol, publicHost, publicPort).First(&service).Error; err != nil {
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
