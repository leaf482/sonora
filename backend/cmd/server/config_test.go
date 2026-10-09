package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	clearDatabaseEnv(t)
	tests := []struct {
		name    string
		value   string
		unset   bool
		want    int
		wantErr bool
	}{
		{name: "unset defaults", unset: true, want: 8080},
		{name: "custom", value: "3000", want: 3000},
		{name: "minimum", value: "1", want: 1},
		{name: "maximum", value: "65535", want: 65535},
		{name: "leading zeros are decimal", value: "08080", want: 8080},
		{name: "empty", value: "", wantErr: true},
		{name: "ephemeral port disallowed", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "too large", value: "65536", wantErr: true},
		{name: "not a number", value: "http", wantErr: true},
		{name: "whitespace", value: " 8080", wantErr: true},
		{name: "sign", value: "+8080", wantErr: true},
		{name: "overflow", value: "999999999999999999999999999", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setenv restores the original environment even for the unset case.
			t.Setenv("PORT", tt.value)
			if tt.unset {
				if err := os.Unsetenv("PORT"); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := loadConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected invalid PORT to fail")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.port != tt.want {
				t.Fatalf("port = %d, want %d", cfg.port, tt.want)
			}
		})
	}
}

func clearDatabaseEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"DATABASE_URL", "DB_CONNECT_TIMEOUT"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDatabaseConfiguration(t *testing.T) {
	t.Setenv("PORT", "8080")
	clearDatabaseEnv(t)
	cfg, err := loadConfig()
	if err != nil || cfg.database != nil || cfg.databaseTimeout != 5*time.Second {
		t.Fatalf("unset database configuration: %v", err)
	}
	for _, tt := range []struct {
		name, raw string
		wantErr   bool
	}{
		{"postgres URL", "postgres://test:secret@127.0.0.1:5432/test?sslmode=disable", false},
		{"postgresql URL", "postgresql://test:secret@localhost/test?sslmode=disable", false},
		{"empty", "", true},
		{"wrong scheme", "https://test:secret@localhost/test", true},
		{"missing host", "postgres://test:secret@/test", true},
		{"missing user", "postgres://localhost/test", true},
		{"missing database", "postgres://test:secret@localhost", true},
		{"bad escape", "postgres://test:secret%@localhost/test", true},
		{"bad port", "postgres://test:secret@localhost:99999/test", true},
		{"bad TLS mode", "postgres://test:secret@localhost/test?sslmode=invalid", true},
		{"fragment", "postgres://test:secret@localhost/test#fragment", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", tt.raw)
			cfg, err := loadConfig()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected invalid DATABASE_URL to fail")
				}
				if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), tt.raw) && tt.raw != "" {
					t.Fatal("configuration error disclosed the connection string")
				}
				return
			}
			if err != nil || cfg.database == nil {
				t.Fatalf("valid database config rejected: %v", err)
			}
			if cfg.database.Database != "test" || cfg.database.User != "test" {
				t.Fatal("database URL was not applied")
			}
		})
	}
	for _, raw := range []string{"", "nope", "0s", "-1s", "31s"} {
		t.Run("timeout "+raw, func(t *testing.T) {
			t.Setenv("DB_CONNECT_TIMEOUT", raw)
			if _, err := loadConfig(); err == nil {
				t.Fatal("expected invalid DB_CONNECT_TIMEOUT to fail")
			}
		})
	}
	t.Setenv("DB_CONNECT_TIMEOUT", "250ms")
	cfg, err = loadConfig()
	if err != nil || cfg.databaseTimeout != 250*time.Millisecond {
		t.Fatalf("custom timeout rejected: %v", err)
	}
}
