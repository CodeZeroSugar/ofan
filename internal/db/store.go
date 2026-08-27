package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(ctx context.Context, dbPath, adminUser, adminPassHash string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(ctx, adminUser, adminPassHash); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migration: %w", err)
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context, adminUser, adminPassHash string) error {
	_, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_root INTEGER NOT NULL DEFAULT 0,
			is_admin INTEGER NOT NULL DEFAULT 0,
			is_suspended INTEGER NOT NULL DEFAULT 0,
			must_change_password INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS servers (
			name TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			config_json TEXT NOT NULL,
			desired_state TEXT NOT NULL DEFAULT 'running',
			purge_storage INTEGER NOT NULL DEFAULT 0,
			consecutive_failures INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (owner) REFERENCES users(username)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create servers table: %w", err)
	}

	isEmpty, err := s.usersEmpty(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if users table is empty: %w", err)
	}
	if isEmpty {
		if err := s.BootstrapAdmin(ctx, adminUser, adminPassHash); err != nil {
			return fmt.Errorf("failed to bootstrap admin: %w", err)
		}
	}
	return nil
}

func (s *Store) usersEmpty(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to count users: %w", err)
	}
	return count == 0, nil
}

func (s *Store) BootstrapAdmin(ctx context.Context, adminUser, adminPassHash string) error {
	_, err := s.db.ExecContext(ctx, `
			INSERT INTO users(
				username,
				password_hash,
				is_root,
				is_admin,
				must_change_password
		)
			VALUES(
				?,
				?,
				1,
				1,
				1
		);
		`, adminUser, adminPassHash)
	if err != nil {
		return fmt.Errorf("failed to bootstrap admin to database: %w", err)
	}
	return nil
}

func (s *Store) ResetDatabase(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM servers
		;`)
	if err != nil {
		return fmt.Errorf("servers table reset failed: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM users
		;`)
	if err != nil {
		return fmt.Errorf("users table reset failed: %w", err)
	}
	return nil
}
