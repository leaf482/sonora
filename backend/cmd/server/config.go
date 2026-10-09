package main

import (
	"errors"
	"os"
	"strconv"
)

type config struct {
	port int
}

func loadConfig() (config, error) {
	raw, set := os.LookupEnv("PORT")
	if !set {
		return config{port: 8080}, nil
	}

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
	return config{port: port}, nil
}
