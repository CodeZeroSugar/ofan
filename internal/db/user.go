package db

import (
	"context"
	"fmt"
	"time"
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
