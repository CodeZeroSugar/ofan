package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Server struct {
	Name      string
	Owner     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrServerNotFound = errors.New("server not found in database")
	ErrServerExists   = errors.New("server is already in database")
)

func (s *Store) serverExists(ctx context.Context, name string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `
		SELECT 1 FROM servers
		WHERE name = ?
		;`, name).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if server '%s' exists: %w", name, err)
	}
	return true, nil
}

func (s *Store) CreateServer(ctx context.Context, name, owner string) error {
	exists, err := s.serverExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return ErrServerExists
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO servers(
		name,
		owner
		)
		VALUES(
		?,
		?
		);`, name, owner)
	if err != nil {
		return fmt.Errorf("failed to create server '%s':%w", name, err)
	}
	return nil
}

func (s *Store) DeleteServer(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM servers
		WHERE name = ?
		;`, name)
	if err != nil {
		return fmt.Errorf("failed to delete server '%s': %w", name, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not verify rows affected: %w", err)
	}
	if n == 0 {
		return ErrServerNotFound
	}
	return nil
}

func (s *Store) GetServerOwner(ctx context.Context, name string) (string, error) {
	var owner string
	err := s.db.QueryRowContext(ctx, `
		SELECT owner
		FROM servers
		WHERE name = ?
		;`, name).Scan(&owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrServerNotFound
		}
		return "", fmt.Errorf("could not get owner for server '%s': %w", name, err)
	}
	return owner, nil
}

func (s *Store) ListServersByOwner(ctx context.Context, owner string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name
		FROM servers
		WHERE owner = ?
		;`, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers for owner '%s': %w", owner, err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan server name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating server rows: %w", err)
	}
	return names, nil
}

func (s *Store) TransferServer(ctx context.Context, name, newOwner string) error {
	exists, err := s.serverExists(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return ErrServerNotFound
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE servers
		SET owner = ?, updated_at = CURRENT_TIMESTAMP
		WHERE name = ?
		;`, newOwner, name)
	if err != nil {
		return fmt.Errorf("failed to transfer ownership for server '%s': %w", name, err)
	}
	return nil
}
