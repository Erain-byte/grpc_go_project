package database

import (
	"net/url"
	"testing"

	"admin/internal/config"
)

func TestGetDSNBuildsPostgresURL(t *testing.T) {
	cfg := config.DatabaseConfig{
		Username:       "admin_service",
		Password:       "p@ss:/word",
		DBName:         "admin",
		SSLMode:        "disable",
		TimeZone:       "Asia/Shanghai",
		ConnectTimeout: "1500ms",
	}

	parsed, err := url.Parse(getDSN(cfg, "127.0.0.1:5433"))
	if err != nil {
		t.Fatalf("parse DSN: %v", err)
	}
	if parsed.Scheme != "postgres" || parsed.Host != "127.0.0.1:5433" || parsed.Path != "/admin" {
		t.Fatalf("unexpected PostgreSQL DSN location: %s", parsed.String())
	}
	password, ok := parsed.User.Password()
	if !ok || parsed.User.Username() != cfg.Username || password != cfg.Password {
		t.Fatalf("credentials were not preserved by URL encoding")
	}
	if got := parsed.Query().Get("connect_timeout"); got != "2" {
		t.Fatalf("connect_timeout = %q, want 2", got)
	}
	if got := parsed.Query().Get("TimeZone"); got != cfg.TimeZone {
		t.Fatalf("TimeZone = %q, want %q", got, cfg.TimeZone)
	}
}
