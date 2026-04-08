package auth

import "os"

type Config struct {
	AdminEmail    string
	AdminPassword string
	JWTSecret     string
}

func ConfigFromEnv() Config {
	return Config{
		AdminEmail:    getEnv("ADMIN_EMAIL", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:     getEnv("JWT_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
