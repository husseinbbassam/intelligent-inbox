package repository

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/husseinbbassam/intelligent-inbox/internal/domain"
)

// NewDB opens a GORM connection to Postgres and auto-migrates the schema.
func NewDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if err = db.AutoMigrate(
		&domain.IngestedRecord{},
		&domain.HumanFeedback{},
	); err != nil {
		return nil, fmt.Errorf("auto-migrate schema: %w", err)
	}

	log.Println("database connection established and schema migrated")
	return db, nil
}
