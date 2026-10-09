package main

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
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
