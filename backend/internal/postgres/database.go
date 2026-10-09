package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func ParseURL(raw string) (*pgx.ConnConfig, error) {
	invalid := errors.New("DATABASE_URL must be a valid postgres or postgresql URL with a host, user and database")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Hostname() == "" || u.User == nil || u.User.Username() == "" ||
		strings.TrimPrefix(u.Path, "/") == "" || u.Fragment != "" {
		return nil, invalid
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		// Driver parse errors can contain the original URL and its password.
		return nil, invalid
	}
	return cfg, nil
}

func Open(ctx context.Context, cfg *pgx.ConnConfig, timeout time.Duration) (*sql.DB, error) {
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		if pingCtx.Err() != nil {
			return nil, fmt.Errorf("database startup ping: %w", pingCtx.Err())
		}
		// Never log the driver error: it can include connection details or
		// server-provided text. Keep a useful, credential-free diagnostic.
		return nil, errors.New("database startup ping failed; check PostgreSQL availability, credentials and DATABASE_URL")
	}
	return db, nil
}

// ConnectTimeout reads the same startup timeout for the server and migration command.
func ConnectTimeout() (time.Duration, error) {
	if raw, set := os.LookupEnv("DB_CONNECT_TIMEOUT"); set {
		timeout, err := time.ParseDuration(raw)
		if err != nil || timeout <= 0 || timeout > 30*time.Second {
			return 0, errors.New("DB_CONNECT_TIMEOUT must be a duration greater than zero and at most 30s")
		}
		return timeout, nil
	}
	return 5 * time.Second, nil
}
