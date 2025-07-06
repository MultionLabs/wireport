package database

import (
	"os"
	"path/filepath"
	"wireport/cmd/server/config"
	join_requests_types "wireport/internal/joinrequests/types"
	"wireport/internal/jointokens"
	"wireport/internal/nodes/types"
	"wireport/internal/publicservices"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func InitDB() (*gorm.DB, error) {
	var err error

	// Ensure the parent directory exists
	dbDir := filepath.Dir(config.Config.DatabasePath)

	if err = os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(config.Config.DatabasePath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&types.Node{}, &join_requests_types.JoinRequest{}, &publicservices.PublicService{}, &jointokens.JoinToken{})

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
