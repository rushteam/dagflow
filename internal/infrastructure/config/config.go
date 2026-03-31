package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mlboy/dagflow/internal/infrastructure/election"
)

type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Database DatabaseConfig  `yaml:"database"`
	Auth     AuthConfig      `yaml:"auth"`
	Election election.Config `yaml:"election"`
}

type ServerConfig struct {
	Port     string `yaml:"port"`
	LogLevel string `yaml:"log_level"`
}

type DatabaseConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type AuthConfig struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	TokenExpiration time.Duration `yaml:"token_expiration"`
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:     ":8080",
			LogLevel: "info",
		},
		Database: DatabaseConfig{
			DSN:             "postgres://dash:dash@localhost:5432/dash?sslmode=disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Auth: AuthConfig{
			JWTSecret:       "change-me-in-production",
			TokenExpiration: 24 * time.Hour,
		},
		Election: election.DefaultConfig(),
	}
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		cfg := defaultConfig()
		return &cfg, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cfg, fmt.Errorf("解析配置文件失败: %w", err)
	}

	cfg.validate()
	return &cfg, nil
}

func (c *Config) validate() {
	if c.Server.Port == "" {
		c.Server.Port = ":8080"
	}
	if c.Auth.JWTSecret == "" {
		slog.Warn("auth.jwt_secret 未设置，使用默认值")
		c.Auth.JWTSecret = "change-me-in-production"
	}
	if c.Auth.TokenExpiration <= 0 {
		c.Auth.TokenExpiration = 24 * time.Hour
	}
}
