package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

type Config struct {
	Host, Port, User, Password, DBName, SSLMode string
}

func ConfigFromEnv() Config {
	return Config{
		Host:     get("DB_HOST", "localhost"),
		Port:     get("DB_PORT", "5432"),
		User:     get("DB_USER", "postgres"),
		Password: get("DB_PASSWORD", "postgres"),
		DBName:   get("DB_NAME", "rag_platform"),
		SSLMode:  get("DB_SSLMODE", "disable"),
	}
}

func get(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}

func Open(ctx context.Context, c Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
	return sql.Open("postgres", dsn)
}