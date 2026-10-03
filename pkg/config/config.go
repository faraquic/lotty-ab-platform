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
	FullName     string `mapstructure:"full_name"`
	Email        string `mapstructure:"email"`
	PasswordHash string `mapstructure:"password_hash"`
}

type DatabaseConfig struct {
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Redis      RedisConfig      `mapstructure:"redis"`
	S3         S3Config         `mapstructure:"s3"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	ClickHouse ClickHouseConfig `mapstructure:"clickhouse"`
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

type KafkaConfig struct {
	Brokers string `mapstructure:"brokers"`
}

type ClickHouseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type PanelConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
}

type RuntimeConfig struct {
	HTTP        HTTPConfig          `mapstructure:"http"`
	MaxStaleAge time.Duration       `mapstructure:"max_stale_age"`
	Kafka       KafkaConsumerConfig `mapstructure:"kafka"`
	// RevalidateInterval is how often the reader compares the Redis revision
	// key with the loaded revision. It closes the window where a snapshot
	// published before the kafka consumer group is assigned is never seen.
	// Zero disables the poll.
	RevalidateInterval time.Duration `mapstructure:"revalidate_interval"`
}

type KafkaConsumerConfig struct {
	GroupID        string `mapstructure:"group_id"`
	DecisionsTopic string `mapstructure:"decisions_topic"`
}

type AnalyticsConfig struct {
	HTTP        HTTPConfig                `mapstructure:"http"`
	Kafka       AnalyticsKafkaConfig      `mapstructure:"kafka"`
	PII         PIIConfig                 `mapstructure:"pii"`
	ClickHouse  AnalyticsClickHouseConfig `mapstructure:"clickhouse"`
	Attribution AttributionConfig         `mapstructure:"attribution"`
}

type AnalyticsKafkaConfig struct {
	GroupID              string `mapstructure:"group_id"`
	PipelineGroupID      string `mapstructure:"pipeline_group_id"`
	IngestGroupID        string `mapstructure:"ingest_group_id"`
	AttributionGroupID   string `mapstructure:"attribution_group_id"`
	EventsTopic          string `mapstructure:"events_topic"`
	ExposuresTopic       string `mapstructure:"exposures_topic"`
	DecisionsTopic       string `mapstructure:"decisions_topic"`
	EventsValidatedTopic string `mapstructure:"events_validated_topic"`
	EventsDedupedTopic   string `mapstructure:"events_deduped_topic"`
	DLQTopic             string `mapstructure:"dlq_topic"`
}

type AnalyticsClickHouseConfig struct {
	BatchSize     int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

type AttributionConfig struct {
	WindowDays          int `mapstructure:"window_days"`
	LateEventGraceHours int `mapstructure:"late_event_grace_hours"`
}

type PIIConfig struct {
	HashSubjectID      bool              `mapstructure:"hash_subject_id"`
	Salts              map[string]string `mapstructure:"salts"`
	CurrentSaltVersion string            `mapstructure:"current_salt_version"`
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
				FullName: "admin",
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
				Endpoint:  "http://localhost:4566",
				AccessKey: "test",
				SecretKey: "test",
			},
			Kafka: KafkaConfig{
				Brokers: "localhost:9092",
			},
			ClickHouse: ClickHouseConfig{
				DSN: "clickhouse://labp:labppassword@localhost:9009/labp",
			},
		},
		Panel: PanelConfig{
			HTTP: HTTPConfig{
				Address: "0.0.0.0:8081",
				CORS: CORSConfig{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
					AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "X-Trace-ID", "Idempotency-Key"},
					ExposeHeaders:    []string{"X-Request-ID", "Idempotent-Replayed"},
					AllowCredentials: true,
					MaxAge:           12 * time.Hour,
				},
				Timeout:         5 * time.Second,
				IdleTimeout:     60 * time.Second,
				ShutdownTimeout: 30 * time.Second,
			},
		},
		Runtime: RuntimeConfig{
			MaxStaleAge:        5 * time.Minute,
			RevalidateInterval: 5 * time.Second,
			Kafka: KafkaConsumerConfig{
				GroupID:        "labp-runtime-snapshot",
				DecisionsTopic: "analytics.decisions.raw",
			},
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
			Kafka: AnalyticsKafkaConfig{
				GroupID:              "labp-analytics",
				PipelineGroupID:      "labp-analytics-pipeline",
				IngestGroupID:        "labp-analytics-ingest",
				AttributionGroupID:   "labp-analytics-attribution",
				EventsTopic:          "analytics.events.raw",
				ExposuresTopic:       "analytics.exposures.raw",
				DecisionsTopic:       "analytics.decisions.raw",
				EventsValidatedTopic: "analytics.events.validated",
				EventsDedupedTopic:   "analytics.events.deduped",
				DLQTopic:             "analytics.dlq",
			},
			PII: PIIConfig{
				HashSubjectID:      true,
				Salts:              map[string]string{"v1": "change-me-analytics-salt-v1"},
				CurrentSaltVersion: "v1",
			},
			ClickHouse: AnalyticsClickHouseConfig{
				BatchSize:     1000,
				FlushInterval: time.Second,
			},
			Attribution: AttributionConfig{
				WindowDays:          7,
				LateEventGraceHours: 1,
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
		if filepath.IsAbs(custom) {
			if st, err := os.Stat(custom); err == nil && !st.IsDir() {
				return custom
			}
			return ""
		}
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

	if salt, ok := cfg.Analytics.PII.Salts[cfg.Analytics.PII.CurrentSaltVersion]; cfg.Analytics.PII.HashSubjectID && (!ok || insecureSecret(salt)) {
		if cfg.Environment == "prod" {
			log.Error(
				"insecure analytics PII salt: set analytics.pii.salts for the current salt version before running in prod",
			)
			os.Exit(1)
		}
		log.Warn("insecure analytics PII salt in use; acceptable only for local development")
	}

	if slices.Contains(cfg.Panel.HTTP.CORS.AllowedOrigins, "*") {
		log.Warn("CORS allowed_origins contains '*': any site can call the API; restrict it in production")
	}
}

func insecureSecret(secret string) bool {
	return secret == "" || strings.HasPrefix(secret, "change-me")
}
