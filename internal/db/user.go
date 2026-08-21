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
		SET is_suspended = 1, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to suspend user '%s': %w", username, err)
	}
	return nil
}

func (s *Store) UnsuspendUser(ctx context.Context, username string) error {
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET is_suspended = 0, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to unsuspend user '%s': %w", username, err)
	}
	return nil
}

func (s *Store) PromoteUser(ctx context.Context, username string) error {
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET is_admin = 1, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to promote user '%s' to admin: %w", username, err)
	}
	return nil
}

func (s *Store) DemoteUser(ctx context.Context, username string) error {
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET is_admin = 0, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND is_root = 0
		;`, username)
	if err != nil {
		return fmt.Errorf("failed to demote user '%s' from admin: %w", username, err)
	}
	return nil
}

func (s *Store) UpdatePassword(ctx context.Context, username, newHash string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, must_change_password = 0, updated_at = CURRENT_TIMESTAMP
		WHERE username = ?
		;`, newHash, username)
	if err != nil {
		return fmt.Errorf("failed to update password for user '%s': %w", username, err)
	}
	return nil
}

func (s *Store) ResetPassword(ctx context.Context, username, newHash string) error {
	isRoot, err := s.isRootUser(ctx, username)
	if err != nil {
		return err
	}
	if isRoot {
		return ErrIsRoot
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = ?, must_change_password = 1, updated_at = CURRENT_TIMESTAMP
		WHERE username = ? AND is_root = 0
		;`, newHash, username)
	if err != nil {
		return fmt.Errorf("failed to initiate password reset for user '%s': %w", username, err)
	}
	return nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, is_root, is_admin, is_suspended, must_change_password, created_at, updated_at  
		FROM users
		WHERE username = ?
		;`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsRoot, &user.IsAdmin, &user.IsSuspended, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user '%s': %w", username, err)
	}
	return &user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, is_root, is_admin, is_suspended, must_change_password, created_at, updated_at  
		FROM users
		WHERE id = ?
		;`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsRoot, &user.IsAdmin, &user.IsSuspended, &user.MustChangePassword, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user '%d': %w", id, err)
	}
	return &user, nil
}
