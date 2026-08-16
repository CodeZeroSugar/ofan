package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ServerRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	NodePort  int32     `json:"node_port"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("record not found")

func (s *Store) CreateServer(ctx context.Context, srv ServerRecord) (*ServerRecord, error) {
	query := `
	INSERT INTO servers (id, name, namespace, node_port, status)
	VALUES (?, ?, ?, ?, ?);
	`
	_, err := s.db.ExecContext(ctx, query, srv.ID, srv.Name, srv.Namespace, srv.NodePort, srv.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to insert server: %w", err)
	}

	row := s.db.QueryRowContext(ctx, query, srv.Name)
	var record ServerRecord

	err = row.Scan(&record.ID, &record.Name, &record.Namespace, &record.NodePort, &record.Status, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query server %s after creation: %w", srv.Name, err)
	}

	return &record, nil
}

func (s *Store) GetServerByName(ctx context.Context, name string) (*ServerRecord, error) {
	query := `SELECT id, name, namespace, node_port, status, created_at, updated_at FROM servers WHERE name = ?;`

	row := s.db.QueryRowContext(ctx, query, name)
	var record ServerRecord

	err := row.Scan(&record.ID, &record.Name, &record.Namespace, &record.NodePort, &record.Status, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query server %s: %w", name, err)
	}
	return &record, nil
}

func (s *Store) UpdateServerStatus(ctx context.Context, name, status string) error {
	query := `UPDATE servers SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?;`

	_, err := s.db.ExecContext(ctx, query, status, name)
	return err
}

func (s *Store) DeleteServer(ctx context.Context, name string) error {
	query := `DELETE FROM servers WHERE name = ?;`
	_, err := s.db.ExecContext(ctx, query, name)
	return err
}
