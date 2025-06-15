package database

import (
	"wireport/cmd/server/config"
	join_requests_types "wireport/internal/join-requests/types"
	"wireport/internal/nodes/types"
	public_services_types "wireport/internal/public-services"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func InitDB() (*gorm.DB, error) {
	var err error

	db, err := gorm.Open(sqlite.Open(config.Config.DatabasePath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&types.Node{})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&join_requests_types.JoinRequest{})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&public_services_types.PublicService{})

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
