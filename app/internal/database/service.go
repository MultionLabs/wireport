package database

import (
	"os"
	"path/filepath"
	"wireport/cmd/server/config"
	join_requests_types "wireport/internal/join-requests/types"
	"wireport/internal/nodes/types"
	public_services_types "wireport/internal/public-services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func InitDB() (*gorm.DB, error) {
	var err error

	// Ensure the parent directory exists
	dbDir := filepath.Dir(config.Config.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(config.Config.DatabasePath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&types.Node{}, &join_requests_types.JoinRequest{}, &public_services_types.PublicService{})

	if err != nil {
		return nil, err
	}

	return db, nil
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()

	if err != nil {
		return err
	}

	return sqlDB.Close()
}
