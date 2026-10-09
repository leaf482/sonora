package main

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type config struct {
	port            int
	database        *pgx.ConnConfig
	databaseTimeout time.Duration
}

func loadConfig() (config, error) {
	cfg := config{port: 8080, databaseTimeout: 5 * time.Second}
	if raw, set := os.LookupEnv("PORT"); set {
		invalid := errors.New("PORT must be a decimal integer between 1 and 65535")
		if raw == "" {
			return config{}, invalid
		}
		for _, char := range raw {
			if char < '0' || char > '9' {
				return config{}, invalid
			}
		}
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return config{}, invalid
		}
		cfg.port = port
	}
	if raw, set := os.LookupEnv("DB_CONNECT_TIMEOUT"); set {
		timeout, err := time.ParseDuration(raw)
		if err != nil || timeout <= 0 || timeout > 30*time.Second {
			return config{}, errors.New("DB_CONNECT_TIMEOUT must be a duration greater than zero and at most 30s")
		}
		cfg.databaseTimeout = timeout
	}
	if raw, set := os.LookupEnv("DATABASE_URL"); set {
		var err error
		cfg.database, err = parseDatabaseURL(raw)
		if err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}
