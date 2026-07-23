package services

import (
	"errors"
	"time"

	"retrobytes/internal/domain"
	"retrobytes/internal/repos"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrBadCreds = errors.New("invalid email or password")

// defaultSessionTTL is used when an AuthService is constructed without an
// explicit TTL (e.g. older callers), keeping behavior sane.
const defaultSessionTTL = 24 * time.Hour

type AuthService struct {
	Users      *repos.UserRepo
	SessionTTL time.Duration
}

func (s *AuthService) ttl() time.Duration {
	if s.SessionTTL <= 0 {
		return defaultSessionTTL
	}
	return s.SessionTTL
}

// Login verifies credentials and, on success, rotates the session id to a
// fresh value (defeating session fixation). It returns the authenticated user
// and the NEW session id, which the caller must write back as the sid cookie.
func (s *AuthService) Login(sid, email, password string) (*domain.User, string, error) {
	u, err := s.Users.ByEmail(email)
	if err != nil {
		return nil, "", ErrBadCreds
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)) != nil {
		return nil, "", ErrBadCreds
	}
	newSID := uuid.NewString()
	if err := s.Users.RotateSession(sid, newSID, u.ID, s.ttl()); err != nil {
		return nil, "", err
	}
	return u, newSID, nil
}

func (s *AuthService) Logout(sid string) error {
	return s.Users.UnbindSession(sid)
}

func (s *AuthService) CurrentUser(sid string) (*domain.User, error) {
	return s.Users.SessionUser(sid)
}
