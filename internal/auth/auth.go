package auth

import (
	"errors"
	"fmt"

	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("cannot hash a blank password")
	}

	params := &argon2id.Params{
		Memory:      65536,
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}

	hash, err := argon2id.CreateHash(password, params)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return hash, nil
}

func ValidatePassword(hash, password string) (bool, error) {
	if hash == "" || password == "" {
		return false, errors.New("cannot validate password with blank hash or password")
	}
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return match, fmt.Errorf("failed to validate password against stored hash: %w", err)
	}
	return match, nil
}
