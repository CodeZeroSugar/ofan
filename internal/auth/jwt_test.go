package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWT_RoundTrip(t *testing.T) {
	mgr := NewManager(nil, []byte("testsecret"))

	tokenString, err := mgr.IssueJWT(1, "bob", 60*time.Minute)
	require.NoError(t, err)

	claims, err := mgr.VerifyJWT(tokenString)
	require.NoError(t, err)
	assert.Equal(t, int64(1), claims.UserID)
	assert.Equal(t, "bob", claims.Username)
}

func TestJWT_ExpireTokenRejected(t *testing.T) {
	mgr := NewManager(nil, []byte("testsecret"))

	badToken, err := mgr.IssueJWT(1, "bob", -60*time.Minute)
	require.NoError(t, err)

	claims, err := mgr.VerifyJWT(badToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWT_TamperedSignature(t *testing.T) {
	mgr := NewManager(nil, []byte("testsecret"))

	tokenString, err := mgr.IssueJWT(1, "bob", 60*time.Minute)
	require.NoError(t, err)
	tokenRune := []rune(tokenString)
	for i := range tokenRune {
		if i%5 == 0 {
			tokenRune[i] = 'a'
		}
	}
	tokenString = string(tokenRune)

	claims, err := mgr.VerifyJWT(tokenString)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWT_WrongSecret(t *testing.T) {
	mgrA := NewManager(nil, []byte("testsecret1"))
	mgrB := NewManager(nil, []byte("testsecret2"))

	tokenString, err := mgrA.IssueJWT(1, "bob", 60*time.Minute)
	require.NoError(t, err)

	claims, err := mgrB.VerifyJWT(tokenString)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

func TestJWT_AlgGuard(t *testing.T) {
	mgr := NewManager(nil, []byte("testsecret"))

	claims := Claims{
		UserID:   1,
		Username: "bob",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	ss, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = mgr.VerifyJWT(ss)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signing method")
}
