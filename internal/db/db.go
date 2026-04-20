package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_initial.sql
var migrationSQL string

//go:embed migrations/002_email_alerts.sql
var migration002SQL string

//go:embed migrations/003_search_response_deadline.sql
var migration003SQL string

//go:embed migrations/004_delivery_unique.sql
var migration004SQL string

//go:embed migrations/005_delivery_status.sql
var migration005SQL string

func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = os.Getenv("GOVSCOUT_DB")
	}
	if path == "" {
		path = "./govscout.db"
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(30000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec(migrationSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate 001: %w", err)
	}

	if _, err := db.Exec(migration002SQL); err != nil {
		// Column may already exist — ignore "duplicate column" errors
		if !isDuplicateColumn(err) {
			db.Close()
			return nil, fmt.Errorf("migrate 002: %w", err)
		}
	}

	if _, err := db.Exec(migration003SQL); err != nil {
		if !isDuplicateColumn(err) {
			db.Close()
			return nil, fmt.Errorf("migrate 003: %w", err)
		}
	}

	if _, err := db.Exec(migration004SQL); err != nil {
		if !isDuplicateColumn(err) {
			db.Close()
			return nil, fmt.Errorf("migrate 004: %w", err)
		}
	}

	if _, err := db.Exec(migration005SQL); err != nil {
		if !isDuplicateColumn(err) {
			db.Close()
			return nil, fmt.Errorf("migrate 005: %w", err)
		}
	}

	return db, nil
}

// Checkpoint runs a WAL truncate checkpoint. Safe to call while other writes
// are in flight; on busy DB it returns the attempted-checkpoint result, not an
// error.
func Checkpoint(database *sql.DB) error {
	_, err := database.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return err
}

func isDuplicateColumn(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}
