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
var ErrGoogleLoginNotAllowed = errors.New("google login is not allowed for this account")
var ErrPasswordTooShort = errors.New("password must be at least 8 characters")

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

func (s *Service) SignupRecruiterLocal(ctx context.Context, password, displayName string, email *string, ipAddress, userAgent *string) (*User, string, error) {
	if len(strings.TrimSpace(password)) < 8 {
		return nil, "", ErrPasswordTooShort
	}

	username, err := s.generateUniqueRecruiterUsername(ctx)
	if err != nil {
		return nil, "", err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	expiresAt := BuildRecruiterExpiry(s.cfg.LocalRecruiterTTLDays)

	userID, err := s.repo.CreateUser(ctx, &User{
		Username:     username,
		DisplayName:  strings.TrimSpace(displayName),
		Email:        email,
		Role:         "recruiter",
		AuthProvider: "local",
		PasswordHash: &hash,
		Status:       "active",
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return nil, "", err
	}

	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}

	return s.createLoginSession(ctx, user, ipAddress, userAgent)
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
			if err := s.repo.LinkGoogleAccount(ctx, userByEmail.ID, googleUser.Sub, googleUser.Email); err != nil {
				return nil, "", err
			}

			user, err = s.repo.FindUserByID(ctx, userByEmail.ID)
			if err != nil {
				return nil, "", err
			}
		} else {
			username, genErr := s.generateUniqueRecruiterUsername(ctx)
			if genErr != nil {
				return nil, "", genErr
			}

			email := googleUser.Email
			displayName := strings.TrimSpace(googleUser.Name)
			if displayName == "" {
				displayName = "Recruiter"
			}
			googleSub := googleUser.Sub
			expiresAt := BuildRecruiterExpiry(s.cfg.GoogleRecruiterTTLDays)

			userID, createErr := s.repo.CreateUser(ctx, &User{
				Username:     username,
				DisplayName:  displayName,
				Email:        &email,
				Role:         "recruiter",
				AuthProvider: "google",
				GoogleSub:    &googleSub,
				Status:       "active",
				ExpiresAt:    expiresAt,
			})
			if createErr != nil {
				return nil, "", createErr
			}

			if linkErr := s.repo.LinkGoogleAccount(ctx, userID, googleSub, email); linkErr != nil {
				return nil, "", linkErr
			}

			user, err = s.repo.FindUserByID(ctx, userID)
			if err != nil {
				return nil, "", err
			}
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

func (s *Service) generateUniqueRecruiterUsername(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		username, err := GenerateRecruiterUsername()
		if err != nil {
			return "", err
		}

		exists, err := s.repo.UsernameExists(ctx, username)
		if err != nil {
			return "", err
		}
		if !exists {
			return username, nil
		}
	}
	return "", errors.New("failed to generate unique recruiter username")
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
