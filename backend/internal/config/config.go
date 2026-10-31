package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config is the root configuration structure for the application.
// All fields are tagged for Viper's mapstructure decoding.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Redis        RedisConfig        `mapstructure:"redis"`
	S3           S3Config           `mapstructure:"s3"`
	OCR          OCRConfig          `mapstructure:"ocr"`
	LLM          LLMConfig          `mapstructure:"llm"`
	JWT          JWTConfig          `mapstructure:"jwt"`
	Webhook      WebhookConfig      `mapstructure:"webhook"`
	SecretMgr    SecretManagerConfig `mapstructure:"secret_manager"`
	HealthChecks HealthCheckConfig  `mapstructure:"health_checks"`
}

// ServerConfig holds HTTP server related settings.
type ServerConfig struct {
	Port         int    `mapstructure:"port"`          // HTTP port
	Env          string `mapstructure:"env"`           // development | production
	ReadTimeout  string `mapstructure:"read_timeout"`  // e.g. "5s"
	WriteTimeout string `mapstructure:"write_timeout"` // e.g. "10s"
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"` // e.g. "postgres://user:pass@host:5432/db?sslmode=disable"
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL string `mapstructure:"url"` // redis://:password@host:6379/0
}

// S3Config holds MinIO / S3 bucket configuration.
type S3Config struct {
	Endpoint        string `mapstructure:"endpoint"`          // http://localhost:9000
	Bucket          string `mapstructure:"bucket"`            // bucket name
	AccessKeyID     string `mapstructure:"access_key_id"`     // may be secret://my-secret-id
	SecretAccessKey string `mapstructure:"secret_access_key"` // may be secret://my-secret-id
	UseSSL          bool   `mapstructure:"use_ssl"`
}

// OCRConfig holds OCR service configuration.
type OCRConfig struct {
	Provider string `mapstructure:"provider"` // "tesseract" or "google_vision"
	Endpoint string `mapstructure:"endpoint"` // for remote services, may be secret://...
}

// LLMConfig holds optional LLM validation service configuration.
type LLMConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"` // may be secret://...
	Model    string `mapstructure:"model"`    // model name
}

// JWTConfig holds JWT signing configuration.
type JWTConfig struct {
	Secret          string `mapstructure:"secret"`            // HMAC secret, may be secret://...
	ExpirationHours int    `mapstructure:"expiration_hours"` // token TTL
}

// WebhookConfig holds defaults for webhook signing.
type WebhookConfig struct {
	SignatureHeader string `mapstructure:"signature_header"` // e.g. "X-FinSight-Signature"
}

// SecretManagerConfig toggles Google Secret Manager usage.
type SecretManagerConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Project string `mapstructure:"project"` // GCP project ID
}

// HealthCheckConfig defines timeout values for health probes.
type HealthCheckConfig struct {
	DatabaseTimeout string `mapstructure:"database_timeout"` // e.g. "2s"
	RedisTimeout    string `mapstructure:"redis_timeout"`    // e.g. "2s"
	OCRTimeout      string `mapstructure:"ocr_timeout"`      // e.g. "5s"
	LLMTimeout      string `mapstructure:"llm_timeout"`      // e.g. "5s"
	QueueTimeout    string `mapstructure:"queue_timeout"`    // e.g. "2s"
}

// Load loads the configuration from file, environment variables and (optionally) Google Secret Manager.
// It returns a fully populated Config ready for consumption.
func Load(logger *zap.Logger) (*Config, error) {
	// -------------------------------------------------------------------------
	// 1. Viper setup – file + env overrides
	// -------------------------------------------------------------------------
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "./config/config.yaml"
	}
	v := viper.New()
	v.SetConfigFile(cfgPath)
	v.SetConfigType("yaml")

	// Environment variables override any config key.
	// Nested keys are accessed with '_' (e.g. SERVER_PORT, DATABASE_DSN)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", cfgPath, err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config into struct: %w", err)
	}

	// -------------------------------------------------------------------------
	// 2. Resolve any secret:// placeholders via Secret Manager (if enabled)
	// -------------------------------------------------------------------------
	if cfg.SecretMgr.Enabled {
		if cfg.SecretMgr.Project == "" {
			return nil, fmt.Errorf("secret_manager.project must be set when secret manager is enabled")
		}
		if err := resolveSecrets(context.Background(), logger, &cfg); err != nil {
			return nil, fmt.Errorf("resolving secrets: %w", err)
		}
	}

	// -------------------------------------------------------------------------
	// 3. Basic validation – ensure required fields are present
	// -------------------------------------------------------------------------
	if err := validate(cfg); err != nil {
		return nil, err
	}

	logger.Info("configuration loaded successfully", zap.Any("config", cfg))
	return &cfg, nil
}

// resolveSecrets walks through the Config struct and replaces any value that
// starts with the prefix "secret://" with the actual secret payload fetched from
// Google Secret Manager.
func resolveSecrets(ctx context.Context, logger *zap.Logger, cfg *Config) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("creating secret manager client: %w", err)
	}
	defer client.Close()

	// Helper to fetch a secret version. By default we read the latest version.
	fetch := func(secretRef string) (string, error) {
		// secretRef format: secret://<secret-id>
		secretID := strings.TrimPrefix(secretRef, "secret://")
		if secretID == "" {
			return "", fmt.Errorf("invalid secret reference %s", secretRef)
		}
		accessReq := &secretmanagerpb.AccessSecretVersionRequest{
			Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", cfg.SecretMgr.Project, secretID),
		}
		result, err := client.AccessSecretVersion(ctx, accessReq)
		if err != nil {
			return "", fmt.Errorf("accessing secret %s: %w", secretID, err)
		}
		return string(result.Payload.Data), nil
	}

	// Resolve each field that may contain a secret reference.
	replaceIfSecret := func(value string) (string, error) {
		if strings.HasPrefix(value, "secret://") {
			secret, err := fetch(value)
			if err != nil {
				return "", err
			}
			logger.Debug("resolved secret", zap.String("ref", value))
			return secret, nil
		}
		return value, nil
	}

	// Database DSN
	if cfg.Database.DSN, err = replaceIfSecret(cfg.Database.DSN); err != nil {
		return fmt.Errorf("database.dsn: %w", err)
	}
	// Redis URL
	if cfg.Redis.URL, err = replaceIfSecret(cfg.Redis.URL); err != nil {
		return fmt.Errorf("redis.url: %w", err)
	}
	// S3 credentials
	if cfg.S3.AccessKeyID, err = replaceIfSecret(cfg.S3.AccessKeyID); err != nil {
		return fmt.Errorf("s3.access_key_id: %w", err)
	}
	if cfg.S3.SecretAccessKey, err = replaceIfSecret(cfg.S3.SecretAccessKey); err != nil {
		return fmt.Errorf("s3.secret_access_key: %w", err)
	}
	// JWT secret
	if cfg.JWT.Secret, err = replaceIfSecret(cfg.JWT.Secret); err != nil {
		return fmt.Errorf("jwt.secret: %w", err)
	}
	// OCR endpoint (optional)
	if cfg.OCR.Endpoint != "" {
		if cfg.OCR.Endpoint, err = replaceIfSecret(cfg.OCR.Endpoint); err != nil {
			return fmt.Errorf("ocr.endpoint: %w", err)
		}
	}
	// LLM endpoint (optional)
	if cfg.LLM.Enabled && cfg.LLM.Endpoint != "" {
		if cfg.LLM.Endpoint, err = replaceIfSecret(cfg.LLM.Endpoint); err != nil {
			return fmt.Errorf("llm.endpoint: %w", err)
		}
	}
	return nil
}

// validate performs a shallow validation of the loaded configuration.
func validate(cfg Config) error {
	if cfg.Server.Port == 0 {
		return fmt.Errorf("server.port must be set and > 0")
	}
	if cfg.Server.Env == "" {
		return fmt.Errorf("server.env must be set (development|production)")
	}
	if cfg.Database.DSN == "" {
		return fmt.Errorf("database.dsn must be set")
	}
	if cfg.Redis.URL == "" {
		return fmt.Errorf("redis.url must be set")
	}
	if cfg.S3.Endpoint == "" || cfg.S3.Bucket == "" {
		return fmt.Errorf("s3.endpoint and s3.bucket must be set")
	}
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret must be set")
	}
	if cfg.JWT.ExpirationHours <= 0 {
		return fmt.Errorf("jwt.expiration_hours must be > 0")
	}
	// Optional: validate time duration strings
	if _, err := time.ParseDuration(cfg.Server.ReadTimeout); err != nil {
		return fmt.Errorf("server.read_timeout invalid duration: %w", err)
	}
	if _, err := time.ParseDuration(cfg.Server.WriteTimeout); err != nil {
		return fmt.Errorf("server.write_timeout invalid duration: %w", err)
	}
	return nil
}

// GetReadTimeout returns the server read timeout as time.Duration.
func (c *Config) GetReadTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Server.ReadTimeout)
	return d
}

// GetWriteTimeout returns the server write timeout as time.Duration.
func (c *Config) GetWriteTimeout() time.Duration {
	d, _ := time.ParseDuration(c.Server.WriteTimeout)
	return d
}

// GetHealthTimeout returns a parsed duration for a given health check key.
// If parsing fails, a sensible default of 2 seconds is returned.
func (c *Config) GetHealthTimeout(key string) time.Duration {
	var raw string
	switch key {
	case "database":
		raw = c.HealthChecks.DatabaseTimeout
	case "redis":
		raw = c.HealthChecks.RedisTimeout
	case "ocr":
		raw = c.HealthChecks.OCRTimeout
	case "llm":
		raw = c.HealthChecks.LLMTimeout
	case "queue":
		raw = c.HealthChecks.QueueTimeout
	default:
		raw = "2s"
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return 2 * time.Second
}