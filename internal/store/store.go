package store

import (
	"database/sql"
	"errors"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registers the "sqlite" driver via init()
)

// Store wraps the SQLite connection.
type Store struct {
	DB *sql.DB
}

// Open opens (or creates) the SQLite database at dsn and applies the schema.
// Use ":memory:" for an in-memory database.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return s, nil
}

// Close closes the underlying connection.
func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	_, err := s.DB.Exec(schemaSQL)
	return err
}
