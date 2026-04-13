package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInactiveUser = errors.New("user is not active")
var ErrExpiredUser = errors.New("user account expired")
var ErrGoogleLoginNotAllowed = errors.New("google login is not allowed for this account")

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

	if err := validateUserForLogin(user); err != nil {
		return nil, "", err
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

	return s.createLoginSession(ctx, user, ipAddress, userAgent)
}

func (s *Service) LoginWithGoogle(ctx context.Context, googleUser *GoogleUserInfo, ipAddress, userAgent *string) (*User, string, error) {
	user, err := s.repo.FindUserByGoogleSub(ctx, googleUser.Sub)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, "", err
	}

	if errors.Is(err, ErrUserNotFound) {
		userByEmail, emailErr := s.repo.FindUserByEmail(ctx, googleUser.Email)
		if emailErr != nil && !errors.Is(emailErr, ErrUserNotFound) {
			return nil, "", emailErr
		}

		if emailErr == nil && userByEmail != nil {
			if userByEmail.Role != "admin" && userByEmail.Role != "recruiter" {
				return nil, "", ErrGoogleLoginNotAllowed
			}
			if userByEmail.Status != "active" {
				return nil, "", ErrInactiveUser
			}

			if err := s.repo.LinkGoogleAccount(ctx, userByEmail.ID, googleUser.Sub, googleUser.Email); err != nil {
				return nil, "", err
			}

			user, err = s.repo.FindUserByID(ctx, userByEmail.ID)
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", ErrGoogleLoginNotAllowed
		}
	}

	if err := validateUserForLogin(user); err != nil {
		return nil, "", err
	}

	return s.createLoginSession(ctx, user, ipAddress, userAgent)
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

	if err := validateUserForLogin(user); err != nil {
		return nil, nil, err
	}

	now := time.Now()
	_ = s.repo.UpdateSessionLastSeen(ctx, session.ID, now)
	_ = s.repo.UpdateUserLastSeen(ctx, user.ID, now)

	return user, session, nil
}

func (s *Service) Logout(ctx context.Context, rawSession string) error {
	return s.repo.RevokeSession(ctx, rawSession, time.Now())
}

func (s *Service) createLoginSession(ctx context.Context, user *User, ipAddress, userAgent *string) (*User, string, error) {
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

func validateUserForLogin(user *User) error {
	if user.Status != "active" {
		if user.Status == "expired" {
			return ErrExpiredUser
		}
		return ErrInactiveUser
	}
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		return ErrExpiredUser
	}
	return nil
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func BuildRecruiterExpiry(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return &t
}

func EnsureGoogleConfigured(cfg Config) error {
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		return fmt.Errorf("google oauth is not configured")
	}
	return nil
}
