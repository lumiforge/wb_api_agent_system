package session

import (
	"fmt"

	adksession "google.golang.org/adk/session"
	adkdatabase "google.golang.org/adk/session/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// PURPOSE: Creates ADK session storage backed by SQLite through GORM.
func NewSQLiteSessionService(sqlitePath string, autoMigrate bool) (adksession.Service, error) {
	baseSessionService, err := adkdatabase.NewSessionService(
		sqlite.Open(sqlitePath),
		&gorm.Config{},
	)
	if err != nil {
		return nil, fmt.Errorf("create sqlite session service: %w", err)
	}

	if autoMigrate {
		// WHY: ADK session tables must exist before the HTTP server starts accepting A2A calls.
		if err := adkdatabase.AutoMigrate(baseSessionService); err != nil {
			return nil, fmt.Errorf("auto migrate session database: %w", err)
		}
	}

	return baseSessionService, nil
}
