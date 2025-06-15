package public_services

import (
	"fmt"
	"time"
)

type PublicService struct {
	LocalProtocol string `gorm:"type:text;not null"`    // http, https, udp, tcp
	LocalHost     string `gorm:"type:text;not null"`    // domain, ip
	LocalPort     uint16 `gorm:"type:integer;not null"` // port

	PublicProtocol string `gorm:"type:text;primaryKey;uniqueIndex:idx_public_service"`    // http, https, udp, tcp
	PublicHost     string `gorm:"type:text;primaryKey;uniqueIndex:idx_public_service"`    // domain:port
	PublicPort     uint16 `gorm:"type:integer;primaryKey;uniqueIndex:idx_public_service"` // port

	CreatedAt time.Time `gorm:"type:timestamp;not null"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null"`
}

func (s *PublicService) AsCaddyConfigEntry() string {
	if s.PublicProtocol == "https" || s.PublicProtocol == "http" {
		return fmt.Sprintf(`
%s://%s {
    reverse_proxy %s://%s:%d
}
		`, s.PublicProtocol, s.PublicHost, s.LocalProtocol, s.LocalHost, s.LocalPort)
	}

	if s.PublicProtocol == "udp" || s.PublicProtocol == "tcp" {
		return fmt.Sprintf(`
%s:%d {
    route {
        proxy {
            upstream %s/%s:%d
        }
    }
}
		`, s.PublicHost, s.PublicPort, s.LocalProtocol, s.LocalHost, s.LocalPort)
	}

	return fmt.Sprintf("# service publication: %s://%s:%d (public) -> %s://%s:%d (local)", s.PublicProtocol, s.PublicHost, s.PublicPort, s.LocalProtocol, s.LocalHost, s.LocalPort)
}
