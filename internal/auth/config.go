package auth

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	AdminEmail             string
	AdminPassword          string
	AdminUsername          string
	SessionCookieName      string
	SessionTTLHours        int
	SecureCookies          bool
	LocalRecruiterTTLDays  int
	GoogleRecruiterTTLDays int

	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURL    string
	GoogleAllowedDomain  string
	FrontendPostLoginURL string

	MFARequiredForAdmin   bool
	MFAIssuer             string
	MFAEmailBackupEnabled bool
	SMTPHost              string
	SMTPPort              int
	SMTPUsername          string
	SMTPPassword          string
	SMTPFrom              string
}

func ConfigFromEnv() Config {
	return Config{
		AdminEmail:             getEnv("ADMIN_EMAIL", ""),
		AdminPassword:          getEnv("ADMIN_PASSWORD", ""),
		AdminUsername:          getEnv("ADMIN_USERNAME", "admin"),
		SessionCookieName:      getEnv("SESSION_COOKIE_NAME", "interview_copilot_session"),
		SessionTTLHours:        getEnvInt("SESSION_TTL_HOURS", 24),
		SecureCookies:          getEnvBool("SECURE_COOKIES", false),
		LocalRecruiterTTLDays:  getEnvInt("LOCAL_RECRUITER_TTL_DAYS", 30),
		GoogleRecruiterTTLDays: getEnvInt("GOOGLE_RECRUITER_TTL_DAYS", 365),

		GoogleClientID:       getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:   getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:    getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/auth/google/callback"),
		GoogleAllowedDomain:  getEnv("GOOGLE_ALLOWED_DOMAIN", ""),
		FrontendPostLoginURL: getEnv("FRONTEND_POST_LOGIN_URL", "http://localhost:5173/"),

		MFARequiredForAdmin:   getEnvBool("MFA_REQUIRED_FOR_ADMIN", false),
		MFAIssuer:             getEnv("MFA_ISSUER", "Interview Copilot"),
		MFAEmailBackupEnabled: getEnvBool("MFA_EMAIL_BACKUP_ENABLED", false),
		SMTPHost:              getEnv("SMTP_HOST", ""),
		SMTPPort:              getEnvInt("SMTP_PORT", 587),
		SMTPUsername:          getEnv("SMTP_USERNAME", ""),
		SMTPPassword:          getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:              getEnv("SMTP_FROM", ""),
	}
}

func (c Config) SessionTTL() time.Duration {
	hours := c.SessionTTLHours
	if hours <= 0 {
		hours = 24
	}
	return time.Duration(hours) * time.Hour
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES":
		return true
	case "0", "false", "FALSE", "no", "NO":
		return false
	default:
		return fallback
	}
}
