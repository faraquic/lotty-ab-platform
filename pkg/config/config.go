package config

import (
	"bytes"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment string         `mapstructure:"environment"`
	Auth        AuthConfig     `mapstructure:"auth"`
	Database    DatabaseConfig `mapstructure:"database"`
	Panel       PanelConfig    `mapstructure:"panel"`
}

type AuthConfig struct {
	JWTConfig JWTConfig `mapstructure:"jwt"`
}

type JWTConfig struct {
	SecretKey string        `mapstructure:"secret_key"`
	TTL       time.Duration `mapstructure:"ttl"`
	RedisTTL  time.Duration `mapstructure:"redis_ttl"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

type PostgresConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int32  `mapstructure:"max_conns"`
	MinConns int32  `mapstructure:"min_conns"`
}

type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type PanelConfig struct {
	HTTPConfig HTTPConfig `mapstructure:"http"`
}

type HTTPConfig struct {
	Address         string        `mapstructure:"address"`
	CORS            CORSConfig    `mapstructure:"cors"`
	Timeout         time.Duration `mapstructure:"timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type CORSConfig struct {
	AllowedOrigins   []string      `mapstructure:"allowed_origins"`
	AllowedMethods   []string      `mapstructure:"allowed_methods"`
	AllowedHeaders   []string      `mapstructure:"allowed_headers"`
	ExposeHeaders    []string      `mapstructure:"expose_headers"`
	AllowCredentials bool          `mapstructure:"allow_credentials"`
	MaxAge           time.Duration `mapstructure:"max_age"`
}

func MustLoad() *Config {
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/labp")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("fatal error config file: %v", err)
	}

	rendered, err := renderEnvTemplate(viper.ConfigFileUsed())
	if err != nil {
		log.Fatalf("unable to render config template: %v", err)
	}

	v := viper.New()
	v.SetConfigType("json")
	if err := v.ReadConfig(bytes.NewReader(rendered)); err != nil {
		log.Fatalf("fatal error config file: %v", err)
	}

	cfg := *defaultConfig()
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	return &cfg
}

func defaultConfig() *Config {
	return &Config{
		Environment: "prod",
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				DSN:      "postgres://lotty:lottypassword@localhost:5433/labp?sslmode=disable",
				MaxConns: 8,
				MinConns: 2,
			},
			Redis: RedisConfig{
				Address:  "localhost:6379",
				DB:       0,
				PoolSize: 8,
			},
		},
		Panel: PanelConfig{
			HTTPConfig{
				Address: "0.0.0.0:8080",
				CORS: CORSConfig{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
					AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
					ExposeHeaders:    []string{"X-Request-ID"},
					AllowCredentials: true,
					MaxAge:           12 * time.Hour,
				},
				Timeout:         5 * time.Second,
				IdleTimeout:     60 * time.Second,
				ShutdownTimeout: 30 * time.Second,
			},
		},
	}
}

func renderEnvTemplate(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	actions := regexp.MustCompile(`\{\{(.*?)\}\}`)
	unescaped := actions.ReplaceAllStringFunc(string(data), func(action string) string {
		return strings.ReplaceAll(action, `\"`, `"`)
	})

	tmpl, err := template.New("config").Funcs(template.FuncMap{
		"ENV": os.Getenv,
	}).Parse(unescaped)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
