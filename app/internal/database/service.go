package database

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"wireport/cmd/server/config"
	join_requests_types "wireport/internal/joinrequests/types"
	"wireport/internal/jointokens"
	"wireport/internal/nodes/types"
	"wireport/internal/publicservices"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

/*
sqliteDSNImmediate returns a DSN that starts write transactions with BEGIN IMMEDIATE so
read-modify-write sequences (e.g. label updates) cannot lose updates under concurrent callers
*/
func sqliteDSNImmediate(path string) string {
	u, err := url.Parse(path)

	if err != nil {
		return path
	}

	q := u.Query()
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()

	return u.String()
}

func InitDB() (*gorm.DB, error) {
	var err error

	// Ensure the parent directory exists
	dbDir := filepath.Dir(config.Config.DatabasePath)

	if err = os.MkdirAll(dbDir, 0755); err != nil {
		return nil, err
	}

	db, err := gorm.Open(sqlite.Open(sqliteDSNImmediate(config.Config.DatabasePath)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})

	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&types.Node{}, &join_requests_types.JoinRequest{}, &publicservices.PublicService{}, &jointokens.JoinToken{})

	if err != nil {
		return nil, err
	}

	if err := ensureNodeLabelsAreValidJSON(db); err != nil {
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

// sets labels to [] when NULL/empty/invalid — JSON serializer always expects valid JSON (not a regular string/null)
func ensureNodeLabelsAreValidJSON(db *gorm.DB) error {
	type nodeLabelRow struct {
		ID     string  `gorm:"column:id"`
		Labels *string `gorm:"column:labels"`
	}

	var rows []nodeLabelRow

	if err := db.Table("nodes").Select("id", "labels").Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		target := []string{}
		needsUpdate := false

		if row.Labels == nil {
			needsUpdate = true
		} else {
			raw := strings.TrimSpace(*row.Labels)
			switch {
			case raw == "":
				needsUpdate = true
			case json.Valid([]byte(raw)):
				needsUpdate = false
			default:
				// Invalid stored value: reset to an empty JSON array.
				needsUpdate = true
			}
		}

		if !needsUpdate {
			continue
		}

		encoded, err := json.Marshal(target)

		if err != nil {
			return err
		}

		if err := db.Table("nodes").Where("id = ?", row.ID).Update("labels", string(encoded)).Error; err != nil {
			return err
		}
	}

	return nil
}
