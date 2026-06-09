package config

import "testing"

func TestLoadMapsServerIngressPathEnv(t *testing.T) {
	t.Setenv("SNMP_BRIDGE_SERVER_INGRESS_PATH", "/api/hassio_ingress/test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.IngressPath != "/api/hassio_ingress/test" {
		t.Fatalf("Server.IngressPath = %q, want %q", cfg.Server.IngressPath, "/api/hassio_ingress/test")
	}
}

func TestDatabaseConfigGetDSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "sqlite returns configured dsn",
			cfg: DatabaseConfig{
				Driver: "sqlite",
				DSN:    "./data/snmp-bridge.db",
			},
			want: "./data/snmp-bridge.db",
		},
		{
			name: "postgres returns explicit dsn",
			cfg: DatabaseConfig{
				Driver: "postgres",
				DSN:    "postgres://snmp_bridge:secret@localhost:5432/snmp_bridge?sslmode=disable",
			},
			want: "postgres://snmp_bridge:secret@localhost:5432/snmp_bridge?sslmode=disable",
		},
		{
			name: "postgres builds dsn with decimal port",
			cfg: DatabaseConfig{
				Driver:   "postgres",
				Host:     "localhost",
				Port:     5432,
				User:     "snmp_bridge",
				Password: "secret",
				DBName:   "snmp_bridge",
			},
			want: "host=localhost port=5432 user=snmp_bridge password=secret dbname=snmp_bridge sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetDSN(); got != tt.want {
				t.Fatalf("GetDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}
