package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInactiveUser = errors.New("user is not active")
var ErrExpiredUser = errors.New("user account expired")

type Service struct {
	cfg  Config
	repo *Repository
}

func NewService(cfg Config, repo *Repository) *Service {
	return &Service{
		cfg:  cfg,
		repo: repo,
	}
}

func (s *Service) EnsureAdminUser(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.AdminUsername) == "" || strings.TrimSpace(s.cfg.AdminPassword) == "" {
		return nil
	}

	_, err := s.repo.FindUserByUsername(ctx, s.cfg.AdminUsername)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return err
	}

	hash, err := HashPassword(s.cfg.AdminPassword)
	if err != nil {
		return err
	}

	var email *string
	if s.cfg.AdminEmail != "" {
		email = &s.cfg.AdminEmail
	}

	_, err = s.repo.CreateUser(ctx, &User{
		Username:     s.cfg.AdminUsername,
		DisplayName:  "Admin",
		Email:        email,
		Role:         "admin",
		AuthProvider: "local",
		PasswordHash: &hash,
		Status:       "active",
	})
	return err
}

func (s *Service) LoginWithPassword(ctx context.Context, username, password string, ipAddress, userAgent *string) (*User, string, error) {
	user, err := s.repo.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if user.Status != "active" {
		if user.Status == "expired" {
			return nil, "", ErrExpiredUser
		}
		return nil, "", ErrInactiveUser
	}

	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		return nil, "", ErrExpiredUser
	}

	if user.PasswordHash == nil || *user.PasswordHash == "" {
		return nil, "", ErrInvalidCredentials
	}

	ok, err := CheckPassword(password, *user.PasswordHash)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrInvalidCredentials
	}

	now := time.Now()
	if err := s.repo.UpdateUserLoginTimestamps(ctx, user.ID, now); err != nil {
		return nil, "", err
	}

	rawSession, err := generateSessionToken()
	if err != nil {
		return nil, "", err
	}

	expiresAt := now.Add(s.cfg.SessionTTL())
	if err := s.repo.CreateSession(ctx, user.ID, rawSession, expiresAt, ipAddress, userAgent); err != nil {
		return nil, "", err
	}

	return user, rawSession, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, rawSession string) (*User, *Session, error) {
	session, err := s.repo.FindSessionByToken(ctx, rawSession)
	if err != nil {
		return nil, nil, err
	}

	if session.RevokedAt != nil {
		return nil, nil, ErrInvalidCredentials
	}
	if session.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrInvalidCredentials
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, nil, err
	}

	if user.Status != "active" {
		return nil, nil, ErrInactiveUser
	}
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		return nil, nil, ErrExpiredUser
	}

	now := time.Now()
	_ = s.repo.UpdateSessionLastSeen(ctx, session.ID, now)
	_ = s.repo.UpdateUserLastSeen(ctx, user.ID, now)

	return user, session, nil
}

func (s *Service) Logout(ctx context.Context, rawSession string) error {
	return s.repo.RevokeSession(ctx, rawSession, time.Now())
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
