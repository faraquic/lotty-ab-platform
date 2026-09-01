package config

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var ServiceName = "labp-"

// ServiceVersion is the build version, overridable at link time via:
//
//	-ldflags "-X github.com/faraquic/lotty-ab-platform/pkg/config.ServiceVersion=v1.12.4+abc1234"
//
// When not injected it defaults to a dev marker.
var ServiceVersion = "dev"

type Config struct {
	Environment string          `mapstructure:"environment"`
	LogLevel    string          `mapstructure:"log_level"`
	Auth        AuthConfig      `mapstructure:"auth"`
	Database    DatabaseConfig  `mapstructure:"database"`
	Panel       PanelConfig     `mapstructure:"panel"`
	Runtime     RuntimeConfig   `mapstructure:"runtime"`
	Analytics   AnalyticsConfig `mapstructure:"analytics"`
}

type AuthConfig struct {
	JWT       JWTConfig       `mapstructure:"jwt"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
}

type JWTConfig struct {
	SecretKey string        `mapstructure:"secret_key"`
	TTL       time.Duration `mapstructure:"ttl"`
}

// BootstrapConfig seeds the first admin when the users table is empty.
type BootstrapConfig struct {
	Username     string `mapstructure:"username"`
	Email        string `mapstructure:"email"`
	PasswordHash string `mapstructure:"password_hash"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	S3       S3Config       `mapstructure:"s3"`
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

type S3Config struct {
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
}

type PanelConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
}

type RuntimeConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
}

type AnalyticsConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
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

func MustLoad(serviceName string) *Config {
	ServiceName += serviceName

	path := findConfigFile()
	if path == "" {
		log.Fatalf(`fatal error config file: not found (looked for $CONFIG_NAME, config.local.json, config.json in "." and "/labp")`)
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("json")

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
		Auth: AuthConfig{
			JWT: JWTConfig{
				SecretKey: "change-me",
				TTL:       18 * time.Hour,
			},
			Bootstrap: BootstrapConfig{
				Username: "admin",
				Email:    "admin@lotty.local",
			},
		},
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
			S3: S3Config{
				Bucket:    "labp",
				Region:    "us-east-1",
				Endpoint:  "http://localhost:9000",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
			},
		},
		Panel: PanelConfig{
			HTTP: HTTPConfig{
				Address: "0.0.0.0:8081",
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
		Runtime: RuntimeConfig{
			HTTP: HTTPConfig{
				Address: "0.0.0.0:8082",
				CORS: CORSConfig{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
					AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
					ExposeHeaders:    []string{"X-Request-ID"},
					AllowCredentials: false,
					MaxAge:           12 * time.Hour,
				},
				Timeout:         5 * time.Second,
				IdleTimeout:     60 * time.Second,
				ShutdownTimeout: 30 * time.Second,
			},
		},
		Analytics: AnalyticsConfig{
			HTTP: HTTPConfig{
				Address: "0.0.0.0:8083",
				CORS: CORSConfig{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
					AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
					ExposeHeaders:    []string{"X-Request-ID"},
					AllowCredentials: false,
					MaxAge:           12 * time.Hour,
				},
				Timeout:         10 * time.Second,
				IdleTimeout:     60 * time.Second,
				ShutdownTimeout: 30 * time.Second,
			},
		},
	}
}

// findConfigFile resolves the config file path:
// $CONFIG_NAME (exact filename or base name) in "." then "/labp",
// otherwise config.local.json, then config.json.
func findConfigFile() string {
	var names []string

	if custom := os.Getenv("CONFIG_NAME"); custom != "" {
		names = append(names, custom)
	}
	names = append(names, "config.local.json", "config.json")

	dirs := []string{".", "/labp"}

	for _, name := range names {
		if !strings.HasSuffix(name, ".json") {
			name += ".json"
		}

		for _, dir := range dirs {
			path := filepath.Join(dir, name)
			if st, err := os.Stat(path); err == nil && !st.IsDir() {
				return path
			}
		}
	}

	return ""
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

func ValidateSecurity(cfg *Config, log *zap.Logger) {
	secret := cfg.Auth.JWT.SecretKey

	if cfg.Environment == "prod" && insecureSecret(secret) {
		log.Error(
			"insecure JWT secret: set auth.jwt.secret_key in config or via ENV template before running in prod",
		)
		os.Exit(1)
	}

	if insecureSecret(secret) {
		log.Warn("insecure JWT secret in use; acceptable only for local development")
	}

	if slices.Contains(cfg.Panel.HTTP.CORS.AllowedOrigins, "*") {
		log.Warn("CORS allowed_origins contains '*': any site can call the API; restrict it in production")
	}
}

func insecureSecret(secret string) bool {
	return secret == "" || strings.HasPrefix(secret, "change-me")
}
