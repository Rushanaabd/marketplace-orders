package db

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"os"
)

func Connect() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_USER", "marketplace"),
			getEnv("DB_PASSWORD", "marketplace"),
			getEnv("DB_NAME", "marketplace_orders"),
			getEnv("DB_SSLMODE", "disable"),
		)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
