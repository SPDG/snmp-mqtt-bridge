package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	MQTT     MQTTConfig     `mapstructure:"mqtt"`
	SNMP     SNMPConfig     `mapstructure:"snmp"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type ServerConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	IngressPath string `mapstructure:"ingress_path"` // Base path for HA Ingress (e.g., /api/hassio_ingress/xxx)
}

type DatabaseConfig struct {
	Driver   string `mapstructure:"driver"` // sqlite or postgres
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type MQTTConfig struct {
	Broker       string `mapstructure:"broker"`
	Port         int    `mapstructure:"port"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	ClientID     string `mapstructure:"client_id"`
	TopicPrefix  string `mapstructure:"topic_prefix"`
	Discovery    bool   `mapstructure:"discovery"`
	DiscoveryPrefix string `mapstructure:"discovery_prefix"`
}

type SNMPConfig struct {
	DefaultCommunity string        `mapstructure:"default_community"`
	DefaultVersion   string        `mapstructure:"default_version"`
	DefaultTimeout   time.Duration `mapstructure:"default_timeout"`
	DefaultRetries   int           `mapstructure:"default_retries"`
	TrapPort         int           `mapstructure:"trap_port"`
	PollInterval     time.Duration `mapstructure:"poll_interval"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load() (*Config, error) {
	v := viper.New()

	// Set config file options
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("/data")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Environment variable support
	v.SetEnvPrefix("SNMP_BRIDGE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Try to read config file (don't fail if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)

	// Database defaults
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "./data/snmp-bridge.db")

	// MQTT defaults
	v.SetDefault("mqtt.broker", "localhost")
	v.SetDefault("mqtt.port", 1883)
	v.SetDefault("mqtt.client_id", "snmp-mqtt-bridge")
	v.SetDefault("mqtt.topic_prefix", "snmp-bridge")
	v.SetDefault("mqtt.discovery", true)
	v.SetDefault("mqtt.discovery_prefix", "homeassistant")

	// SNMP defaults
	v.SetDefault("snmp.default_community", "public")
	v.SetDefault("snmp.default_version", "v2c")
	v.SetDefault("snmp.default_timeout", "5s")
	v.SetDefault("snmp.default_retries", 3)
	v.SetDefault("snmp.trap_port", 162)
	v.SetDefault("snmp.poll_interval", "30s")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
}

// GetDSN returns the database connection string
func (c *DatabaseConfig) GetDSN() string {
	if c.Driver == "sqlite" {
		return c.DSN
	}
	// PostgreSQL DSN
	if c.DSN != "" {
		return c.DSN
	}
	return "host=" + c.Host + " port=" + string(rune(c.Port)) + " user=" + c.User + " password=" + c.Password + " dbname=" + c.DBName + " sslmode=disable"
}
