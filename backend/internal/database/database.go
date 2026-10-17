package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"flowmonitor/backend/internal/models"
)

// Connect opens the SQLite database (pure-Go driver, no CGO required).
func Connect(dbPath string) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access underlying sql db: %w", err)
	}
	// SQLite works best with a single writer connection.
	sqlDB.SetMaxOpenConns(1)

	return db, nil
}

// Migrate runs auto-migration for all domain models.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.Flow{},
		&models.FlowNode{},
		&models.FlowEdge{},
		&models.Run{},
		&models.StepRun{},
		&models.Event{},
		&models.Alert{},
	)
}
