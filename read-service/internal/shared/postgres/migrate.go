package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// NOTE: goose uses database/sql
// We keep runtime DB access on pgxpool, and migrations on stdlib adapter.
// This is a common, pragmatic approach in Go services.
func OpenStdlib(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open stdlib db: %w", err)
	}
	return db, nil
}
