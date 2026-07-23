package services

import (
	"errors"

	"retrobytes/internal/domain"
	"retrobytes/internal/repos"

	"golang.org/x/crypto/bcrypt"
)

var ErrBadCreds = errors.New("invalid email or password")

type AuthService struct {
	Users *repos.UserRepo
}

func (s *AuthService) Login(sid, email, password string) (*domain.User, error) {
	u, err := s.Users.ByEmail(email)
	if err != nil {
		return nil, ErrBadCreds
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)) != nil {
		return nil, ErrBadCreds
	}
	if err := s.Users.BindSession(sid, u.ID); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *AuthService) Logout(sid string) error {
	return s.Users.UnbindSession(sid)
}

func (s *AuthService) CurrentUser(sid string) (*domain.User, error) {
	return s.Users.SessionUser(sid)
}
