package main

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/leaf482/sonora/backend/internal/postgres"
)

type config struct {
	port            int
	database        *pgx.ConnConfig
	databaseTimeout time.Duration
}

func loadConfig() (config, error) {
	cfg := config{port: 8080}
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
	var err error
	cfg.databaseTimeout, err = postgres.ConnectTimeout()
	if err != nil {
		return config{}, err
	}
	if raw, set := os.LookupEnv("DATABASE_URL"); set {
		var err error
		cfg.database, err = postgres.ParseURL(raw)
		if err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}
