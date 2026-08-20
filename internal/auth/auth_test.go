package auth

import (
	"testing"

	"github.com/alexedwards/argon2id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	// Test: valid password
	p := "secret123"
	hash, err := HashPassword(p)
	require.NoError(t, err)
	assert.NotEqual(t, p, hash)

	// Test: two hashes of the same passwor differ
	p = "secret123"
	hash1, err := HashPassword(p)
	require.NoError(t, err)
	hash2, err := HashPassword(p)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash2)

	// Test: Blank password
	p = ""
	_, err = HashPassword(p)
	require.Error(t, err)

	// Test: Output is self-describing
	p = "argon2idisawesome"
	hash, err = HashPassword(p)
	require.NoError(t, err)
	assert.Contains(t, hash, "$argon2id$")
}

func TestValidatePassword(t *testing.T) {
	// Test: correct password
	p := "secret123"
	hash, err := HashPassword(p)
	require.NoError(t, err)

	isValid, err := ValidatePassword(hash, p)
	assert.NoError(t, err)
	assert.True(t, isValid)

	// Test: Wrong password
	p = "secret123"
	hash, err = HashPassword(p)
	require.NoError(t, err)

	isValid, err = ValidatePassword(hash, "wrongpasswordoopsie")
	assert.NoError(t, err)
	assert.False(t, isValid)

	// Test: Validated against a hash of a different password
	p1 := "secret123"
	p2 := "secret321"

	hash1, err := HashPassword(p1)
	require.NoError(t, err)
	hash2, err := HashPassword(p2)
	require.NoError(t, err)

	isValid, err = ValidatePassword(hash1, hash2)
	assert.NoError(t, err)
	assert.False(t, isValid)

	// Test: blank hash
	isValid, err = ValidatePassword("", "secret123")
	require.Error(t, err)
	assert.False(t, isValid)

	// Test: blank password
	p = "secret123"
	hash, err = HashPassword(p)
	require.NoError(t, err)

	isValid, err = ValidatePassword(hash, "")
	require.Error(t, err)
	assert.False(t, isValid)

	// Test: malformed hash string
	isValid, err = ValidatePassword("not-a-valid-hash", "secret123")
	require.ErrorIs(t, err, argon2id.ErrInvalidHash)
	assert.False(t, isValid)
}
