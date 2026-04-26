package sqlite

import (
	"context"
	"fmt"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// PURPOSE: Opens the shared SQLite database used by app-owned repositories.
func NewDB(sqlitePath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	return db, nil
}

func ApplyMigrationFile(ctx context.Context, db *gorm.DB, path string) error {
	sqlBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	// WHY: Registry tables are app-owned and must exist before indexing YAML files.
	if err := db.WithContext(ctx).Exec(string(sqlBytes)).Error; err != nil {
		return fmt.Errorf("apply migration file: %w", err)
	}

	return nil
}
