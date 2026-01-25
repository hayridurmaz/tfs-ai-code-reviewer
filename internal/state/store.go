package state

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS reviews (
		repo_id TEXT,
		pr_id INTEGER,
		iteration_id INTEGER,
		reviewed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		result TEXT,
		PRIMARY KEY (repo_id, pr_id, iteration_id)
	);`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func (s *Store) IsIterationReviewed(repoID string, prID int, iterationID int) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM reviews WHERE repo_id = ? AND pr_id = ? AND iteration_id = ?)"
	err := s.db.QueryRow(query, repoID, prID, iterationID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if iteration reviewed: %w", err)
	}
	return exists, nil
}

func (s *Store) MarkIterationReviewed(repoID string, prID int, iterationID int, result string) error {
	query := "INSERT OR REPLACE INTO reviews (repo_id, pr_id, iteration_id, result) VALUES (?, ?, ?, ?)"
	_, err := s.db.Exec(query, repoID, prID, iterationID, result)
	if err != nil {
		return fmt.Errorf("failed to mark iteration reviewed: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
