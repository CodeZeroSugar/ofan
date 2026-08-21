package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrIsRoot       = errors.New("cannot modify root user")
	ErrUserNotFound = errors.New("could not find user for operation")
)

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
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		DELETE FROM users
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to delete user '%s' from database: %w", username, err)
	}
	return nil
}

func (s *Store) SuspendUser(ctx context.Context, username string) error {
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET is_suspended = 1
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to suspend user '%s': %w", username, err)
	}
	return nil
}

func (s *Store) isRootUser(ctx context.Context, username string) (bool, error) {
	var isRoot bool
	err := s.db.QueryRowContext(ctx, `
		SELECT is_root FROM users WHERE username = ?
		;`, username).Scan(&isRoot)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrUserNotFound
		}
		return false, fmt.Errorf("failed to look up user: %w", err)
	}
	return isRoot, nil
}
