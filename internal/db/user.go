package db

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrIsRoot = errors.New("cannot delete the root user")

type User struct {
	ID                 int64
	Username           string
	PasswordHash       string
	IsRoot             bool
	IsAdmin            bool
	IsSuspended        bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, isAdmin bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users(
		username,
		password_hash,
		is_admin,
		must_change_password
		)
		VALUES(
		?,
		?,
		?,
		1
		);
		`, username, passwordHash, isAdmin)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, username string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM users
		WHERE username = ? AND is_root = 0;
		`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user '%s' from database: %w", username, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrIsRoot
	}
	return nil
}
