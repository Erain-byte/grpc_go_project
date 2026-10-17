package config

import "testing"

func TestUserPostgresAndShardingConfig(t *testing.T) {
	cfg, err := InitConfig("../../etc/user.yaml")
	if err != nil {
		t.Fatalf("load user config: %v", err)
	}

	if cfg.Database.Driver != "postgres" {
		t.Fatalf("database driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.Port != 5434 {
		t.Fatalf("database port = %d, want 5434", cfg.Database.Port)
	}
	if !cfg.Database.Sharding.Enabled {
		t.Fatal("user sharding must be enabled")
	}
	if cfg.Database.Sharding.TableCount != 8 {
		t.Fatalf("shard table count = %d, want 8", cfg.Database.Sharding.TableCount)
	}
	if cfg.Database.Sharding.TablePrefix != "users_" {
		t.Fatalf("shard table prefix = %q, want users_", cfg.Database.Sharding.TablePrefix)
	}
}

func TestValidateRejectsChangedUserShardCount(t *testing.T) {
	cfg, err := InitConfig("../../etc/user.yaml")
	if err != nil {
		t.Fatalf("load user config: %v", err)
	}
	cfg.Database.Sharding.TableCount = 16

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted a changed user shard count")
	}
}
