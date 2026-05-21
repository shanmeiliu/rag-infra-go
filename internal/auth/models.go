package auth

import "time"

type User struct {
	ID              int64      `json:"id"`
	Username        string     `json:"username"`
	DisplayName     string     `json:"display_name"`
	Email           *string    `json:"email,omitempty"`
	Role            string     `json:"role"`
	AuthProvider    string     `json:"auth_provider"`
	PasswordHash    *string    `json:"-"`
	GoogleSub       *string    `json:"-"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	InvitedByUserID *int64     `json:"-"`
	Notes           *string    `json:"notes,omitempty"`
	MFAEnabled      bool       `json:"mfa_enabled"`
	MFATOTPSecret   *string    `json:"-"`
	MFAConfirmedAt  *time.Time `json:"mfa_confirmed_at,omitempty"`
	MFAEmailEnabled bool       `json:"mfa_email_enabled"`
	MFAEmail        *string    `json:"mfa_email,omitempty"`
}

type Session struct {
	ID               int64
	UserID           int64
	SessionTokenHash string
	CreatedAt        time.Time
	LastSeenAt       time.Time
	ExpiresAt        time.Time
	IPAddress        *string
	UserAgent        *string
	RevokedAt        *time.Time
}
