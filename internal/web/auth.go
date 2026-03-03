package web

import (
	"crypto/sha256"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/theognis1002/govscout/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionName    = "govscout"
	sessionMaxAge  = 86400 // 24 hours
	cookieUserID   = "user_id"
	cookieUsername = "username"
	cookieIsAdmin  = "is_admin"
)

type contextKey string

const userContextKey contextKey = "user"

type SessionUser struct {
	ID       int64
	Username string
	IsAdmin  bool
}

func newSecureCookie(secret string) *securecookie.SecureCookie {
	hmacKey := sha256.Sum256([]byte("hmac:" + secret))
	encKey := sha256.Sum256([]byte("encrypt:" + secret))
	sc := securecookie.New(hmacKey[:], encKey[:16])
	sc.MaxAge(sessionMaxAge)
	return sc
}

func (s *Server) setSession(w http.ResponseWriter, user *db.UserRow) {
	value := map[string]any{
		cookieUserID:   user.ID,
		cookieUsername: user.Username,
		cookieIsAdmin:  user.IsAdmin,
	}
	encoded, err := s.cookie.Encode(sessionName, value)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func (s *Server) getSession(r *http.Request) *SessionUser {
	cookie, err := r.Cookie(sessionName)
	if err != nil {
		return nil
	}
	value := map[string]any{}
	if err := s.cookie.Decode(sessionName, cookie.Value, &value); err != nil {
		return nil
	}

	// securecookie decodes integers as json.Number or float64
	userID := toInt64(value[cookieUserID])
	username, _ := value[cookieUsername].(string)
	isAdmin, _ := value[cookieIsAdmin].(bool)

	if userID == 0 || username == "" {
		return nil
	}
	return &SessionUser{ID: userID, Username: username, IsAdmin: isAdmin}
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.getSession(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := r.Context()
		ctx = setUser(ctx, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	default:
		return 0
	}
}
