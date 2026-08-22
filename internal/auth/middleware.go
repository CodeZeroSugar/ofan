package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/CodeZeroSugar/ofan/internal/db"
)

type ctxKey int

const userKey ctxKey = 0

func getBearerToken(headers http.Header) (string, error) {
	a := headers.Get("Authorization")
	if len(a) == 0 {
		return a, fmt.Errorf("'Authorization' does not exist in header")
	}
	tokenString, found := strings.CutPrefix(a, "Bearer ")
	if !found {
		return "", errors.New("invalid Authorization header")
	}
	return strings.TrimSpace(tokenString), nil
}

func (m *Manager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := getBearerToken(r.Header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		claims, err := m.VerifyJWT(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		user, err := m.store.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, db.ErrUserNotFound) {
				http.Error(w, err.Error(), http.StatusUnauthorized)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		if user.IsSuspended {
			http.Error(w, "authentication rejected, please talk to an admin", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, user)
		if user.MustChangePassword && (r.URL.Path != "/api/v1/auth/password" && r.URL.Path != "/api/v1/auth/logout") {
			http.Error(w, "must change password", http.StatusForbidden)
			return
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "user not in context", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "user not in context", http.StatusUnauthorized)
			return
		}

		if !user.IsAdmin && !user.IsRoot {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireRoot(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "user not in context", http.StatusUnauthorized)
			return
		}

		if !user.IsRoot {
			http.Error(w, "insufficient permissions", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(userKey).(*db.User)
	return u
}
