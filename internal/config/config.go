// Package config config.go - 配置管理
// 加载 YAML 配置文件，支持环境变量和命令行参数覆盖
package config

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kyangconn/music-online-go/internal/domain"
	"github.com/kyangconn/music-online-go/internal/pkg/password"
	"github.com/spf13/viper"
)

type Config struct {
	SourceFile     string
	Server         ServerConfig
	Database       DatabaseConfig
	JWT            JWTConfig
	Metrics        MetricsConfig
	AdminBootstrap AdminBootstrapConfig
	Access         AccessConfig
	Library        LibraryConfig
	Classification ClassificationConfig
	Integrations   IntegrationsConfig
	RateLimit      RateLimitConfig
	Logging        LoggingConfig
}

type ServerConfig struct {
	ListenAddress     string
	Port              string
	Mode              string
	ReadHeaderTimeout int
	ReadTimeout       int
	WriteTimeout      int
	IdleTimeout       int
	ShutdownTimeout   int
	ReadinessTimeout  int
	UploadDir         string
	LogFile           string
	MaxJSONBodySizeMB int
	MaxAudioSizeMB    int
	MaxCoverSizeMB    int
	AllowedOrigins    []string
	TrustedProxies    []string
}

type DatabaseConfig struct {
	Type                         string // postgres / sqlite
	Host                         string
	Port                         string
	User                         string
	Password                     string
	Name                         string
	SSLMode                      string
	LogLevel                     string
	Path                         string // sqlite 文件路径
	ConnectTimeoutSeconds        int
	MaxOpenConnections           int
	MaxIdleConnections           int
	ConnectionMaxLifetimeMinutes int
	ConnectionMaxIdleTimeMinutes int
}

type JWTConfig struct {
	Secret string
	// AccessTokenTTLMinutes is the lifetime of the short-lived access token.
	AccessTokenTTLMinutes int
	// RefreshTokenTTLDays is the lifetime of a server-side session (refresh token).
	RefreshTokenTTLDays int
	// RefreshCookieSecure sets the Secure attribute on the refresh httpOnly cookie.
	// Enable it only when the instance is served over HTTPS.
	RefreshCookieSecure bool
}

type MetricsConfig struct {
	Enabled bool
	Token   string
}

type AdminBootstrapConfig struct {
	Enabled       bool
	Username      string
	Email         string
	Password      string
	FullName      string
	ResetPassword bool
}

const (
	LibraryAccessPublic        = "public"
	LibraryAccessAuthenticated = "authenticated"
	RegistrationOpen           = "open"
	RegistrationAdmin          = "admin"
	DefaultMaxAudioSizeMB      = 200
	DefaultMaxCoverSizeMB      = 10
)

type AccessConfig struct {
	LibraryMode        string
	RegistrationMode   string
	MediaURLTTLMinutes int
}

type LibraryConfig struct {
	Scanner                    LibraryScannerConfig
	HealthCheckIntervalSeconds int
}

type LibraryScannerConfig struct {
	Enabled             bool
	MaxFileSizeMB       int
	MaxTagSizeMB        int
	MinFileAgeSeconds   int
	HashRecheckHours    int
	RetryMaxAttempts    int
	RetryInitialSeconds int
	RetryMaxSeconds     int
}

// ClassificationConfig controls the cheap, local metadata rule layer. Audio
// analysis has its own optional configuration so disabling an external
// analyzer never disables tag-based classification or local playback.
type ClassificationConfig struct {
	Enabled            bool
	AnalyzeOnUpload    bool
	AutoThreshold      float64
	ReviewMargin       float64
	CalmFlowWeight     float64
	KineticPulseWeight float64
	CosmicDriftWeight  float64
	BassImpactWeight   float64
	Analyzer           AnalyzerConfig
}

type AnalyzerConfig struct {
	Mode                string
	Endpoint            string
	Token               string
	ID                  string
	Version             string
	ModelVersion        string
	TimeoutSeconds      int
	Concurrency         int
	QueueLimit          int
	MaxFileSizeMB       int
	MaxDurationSeconds  int
	RetryMaxAttempts    int
	RetryInitialSeconds int
	RetryMaxSeconds     int
}

type IntegrationsConfig struct {
	MusicBee MusicBeeConfig
}

type MusicBeeConfig struct {
	SubmitToken    string
	SubmitUsername string
}

type RateLimitConfig struct {
	Enabled                 bool
	GlobalRequestsPerSecond float64
	GlobalBurst             int
	AuthRequestsPerSecond   float64
	AuthBurst               int
}

type LoggingConfig struct {
	Level      string
	AccessLog  bool
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
	LocalTime  bool
}

var AppConfig *Config

// DefaultConfig is the single source of non-secret configuration defaults.
// Required secrets intentionally remain empty: callers that construct a
// configuration programmatically must supply them before validation.
func DefaultConfig() Config {
	presetDefaults := domain.DefaultPresetRulePolicy()
	return Config{
		Server: ServerConfig{
			Port:              "8080",
			Mode:              "debug",
			ReadHeaderTimeout: 10,
			IdleTimeout:       60,
			ShutdownTimeout:   15,
			ReadinessTimeout:  2,
			UploadDir:         "uploads",
			MaxJSONBodySizeMB: 1,
			MaxAudioSizeMB:    DefaultMaxAudioSizeMB,
			MaxCoverSizeMB:    DefaultMaxCoverSizeMB,
			AllowedOrigins:    []string{},
			TrustedProxies:    []string{},
		},
		Database: DatabaseConfig{
			Type:                         "sqlite",
			Port:                         "5432",
			SSLMode:                      "prefer",
			LogLevel:                     "auto",
			Path:                         "music.db",
			ConnectTimeoutSeconds:        10,
			ConnectionMaxLifetimeMinutes: 60,
			ConnectionMaxIdleTimeMinutes: 10,
		},
		JWT: JWTConfig{
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   30,
		},
		AdminBootstrap: AdminBootstrapConfig{
			FullName: "Administrator",
		},
		Access: AccessConfig{
			LibraryMode:        LibraryAccessPublic,
			RegistrationMode:   RegistrationOpen,
			MediaURLTTLMinutes: 60,
		},
		Library: LibraryConfig{
			HealthCheckIntervalSeconds: 60,
			Scanner: LibraryScannerConfig{
				Enabled:             true,
				MaxFileSizeMB:       2048,
				MaxTagSizeMB:        16,
				MinFileAgeSeconds:   30,
				HashRecheckHours:    168,
				RetryMaxAttempts:    5,
				RetryInitialSeconds: 30,
				RetryMaxSeconds:     900,
			},
		},
		Classification: ClassificationConfig{
			Enabled:            presetDefaults.Enabled,
			AutoThreshold:      presetDefaults.AutoThreshold,
			ReviewMargin:       presetDefaults.ReviewMargin,
			CalmFlowWeight:     presetDefaults.CalmFlowWeight,
			KineticPulseWeight: presetDefaults.KineticPulseWeight,
			CosmicDriftWeight:  presetDefaults.CosmicDriftWeight,
			BassImpactWeight:   presetDefaults.BassImpactWeight,
			Analyzer: AnalyzerConfig{
				Mode:                "disabled",
				TimeoutSeconds:      300,
				Concurrency:         1,
				QueueLimit:          1000,
				MaxFileSizeMB:       2048,
				MaxDurationSeconds:  1800,
				RetryMaxAttempts:    3,
				RetryInitialSeconds: 30,
				RetryMaxSeconds:     900,
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:                 true,
			GlobalRequestsPerSecond: 20,
			GlobalBurst:             50,
			AuthRequestsPerSecond:   1,
			AuthBurst:               5,
		},
		Logging: LoggingConfig{
			Level:      "info",
			AccessLog:  true,
			MaxSizeMB:  50,
			MaxBackups: 3,
			MaxAgeDays: 28,
			Compress:   true,
			LocalTime:  true,
		},
	}
}

func (cfg ClassificationConfig) PresetRulePolicy() domain.PresetRulePolicy {
	return domain.PresetRulePolicy{
		Enabled:            cfg.Enabled,
		AutoThreshold:      cfg.AutoThreshold,
		ReviewMargin:       cfg.ReviewMargin,
		CalmFlowWeight:     cfg.CalmFlowWeight,
		KineticPulseWeight: cfg.KineticPulseWeight,
		CosmicDriftWeight:  cfg.CosmicDriftWeight,
		BassImpactWeight:   cfg.BassImpactWeight,
	}
}

func LoadConfig() error {
	// A private Viper instance prevents tests or future reload attempts from
	// mutating the process-wide parser while the validated config is in use.
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	// Viper 读取第一个找到的文件，因此按优先级从高到低添加搜索路径。
	addSearchPaths(v)

	// 环境变量覆盖: MO_CONFIG_FILE
	if envPath := os.Getenv("MO_CONFIG_FILE"); envPath != "" {
		v.SetConfigFile(envPath)
	}

	setViperDefaults(v, DefaultConfig())

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return fmt.Errorf("read config file: %w", err)
		}
	}

	logFile := os.Getenv("MO_LOG_FILE")
	if logFile == "" {
		logFile = v.GetString("server.log_file")
	}
	allowedOrigins, err := NormalizeAllowedOrigins(v.GetStringSlice("server.allowed_origins"))
	if err != nil {
		return err
	}
	trustedProxies, err := NormalizeTrustedProxies(v.GetStringSlice("server.trusted_proxies"))
	if err != nil {
		return err
	}
	databasePassword, err := secretValueFrom(v, "database.password", "DATABASE_PASSWORD")
	if err != nil {
		return err
	}
	jwtSecret, err := secretValueFrom(v, "jwt.secret", "JWT_SECRET")
	if err != nil {
		return err
	}
	metricsToken, err := secretValueFrom(v, "metrics.token", "METRICS_TOKEN")
	if err != nil {
		return err
	}
	bootstrapPassword, err := secretValueFrom(v, "admin.bootstrap.password", "ADMIN_BOOTSTRAP_PASSWORD")
	if err != nil {
		return err
	}
	musicBeeSubmitToken, err := secretValueFrom(v, "integrations.musicbee.submit_token", "INTEGRATIONS_MUSICBEE_SUBMIT_TOKEN")
	if err != nil {
		return err
	}
	analyzerToken, err := secretValueFrom(v, "classification.analyzer.token", "CLASSIFICATION_ANALYZER_TOKEN")
	if err != nil {
		return err
	}

	loaded := &Config{
		SourceFile: v.ConfigFileUsed(),
		Server: ServerConfig{
			ListenAddress:     v.GetString("server.listen_address"),
			Port:              v.GetString("server.port"),
			Mode:              strings.ToLower(strings.TrimSpace(v.GetString("server.mode"))),
			ReadHeaderTimeout: v.GetInt("server.read_header_timeout"),
			ReadTimeout:       v.GetInt("server.read_timeout"),
			WriteTimeout:      v.GetInt("server.write_timeout"),
			IdleTimeout:       v.GetInt("server.idle_timeout"),
			ShutdownTimeout:   v.GetInt("server.shutdown_timeout"),
			ReadinessTimeout:  v.GetInt("server.readiness_timeout"),
			UploadDir:         v.GetString("server.upload_dir"),
			LogFile:           logFile,
			MaxJSONBodySizeMB: v.GetInt("server.max_json_body_size_mb"),
			MaxAudioSizeMB:    v.GetInt("server.max_audio_size_mb"),
			MaxCoverSizeMB:    v.GetInt("server.max_cover_size_mb"),
			AllowedOrigins:    allowedOrigins,
			TrustedProxies:    trustedProxies,
		},
		Database: DatabaseConfig{
			Type:                         strings.ToLower(strings.TrimSpace(v.GetString("database.type"))),
			Host:                         v.GetString("database.host"),
			Port:                         strings.TrimSpace(v.GetString("database.port")),
			User:                         v.GetString("database.user"),
			Password:                     databasePassword,
			Name:                         v.GetString("database.name"),
			SSLMode:                      strings.ToLower(strings.TrimSpace(v.GetString("database.sslmode"))),
			LogLevel:                     strings.ToLower(strings.TrimSpace(v.GetString("database.log_level"))),
			Path:                         v.GetString("database.path"),
			ConnectTimeoutSeconds:        v.GetInt("database.connect_timeout_seconds"),
			MaxOpenConnections:           v.GetInt("database.max_open_connections"),
			MaxIdleConnections:           v.GetInt("database.max_idle_connections"),
			ConnectionMaxLifetimeMinutes: v.GetInt("database.connection_max_lifetime_minutes"),
			ConnectionMaxIdleTimeMinutes: v.GetInt("database.connection_max_idle_time_minutes"),
		},
		JWT: JWTConfig{
			Secret:                jwtSecret,
			AccessTokenTTLMinutes: v.GetInt("jwt.access_token_ttl_minutes"),
			RefreshTokenTTLDays:   v.GetInt("jwt.refresh_token_ttl_days"),
			RefreshCookieSecure:   v.GetBool("jwt.refresh_cookie_secure"),
		},
		Metrics: MetricsConfig{
			Enabled: v.GetBool("metrics.enabled"),
			Token:   metricsToken,
		},
		AdminBootstrap: AdminBootstrapConfig{
			Enabled:       v.GetBool("admin.bootstrap.enabled"),
			Username:      strings.TrimSpace(v.GetString("admin.bootstrap.username")),
			Email:         strings.TrimSpace(v.GetString("admin.bootstrap.email")),
			Password:      bootstrapPassword,
			FullName:      strings.TrimSpace(v.GetString("admin.bootstrap.full_name")),
			ResetPassword: v.GetBool("admin.bootstrap.reset_password"),
		},
		Access: AccessConfig{
			LibraryMode:        strings.ToLower(strings.TrimSpace(v.GetString("access.library_mode"))),
			RegistrationMode:   strings.ToLower(strings.TrimSpace(v.GetString("access.registration_mode"))),
			MediaURLTTLMinutes: v.GetInt("access.media_url_ttl_minutes"),
		},
		Library: LibraryConfig{
			HealthCheckIntervalSeconds: v.GetInt("library.health_check_interval_seconds"),
			Scanner: LibraryScannerConfig{
				Enabled:             v.GetBool("library.scanner.enabled"),
				MaxFileSizeMB:       v.GetInt("library.scanner.max_file_size_mb"),
				MaxTagSizeMB:        v.GetInt("library.scanner.max_tag_size_mb"),
				MinFileAgeSeconds:   v.GetInt("library.scanner.min_file_age_seconds"),
				HashRecheckHours:    v.GetInt("library.scanner.hash_recheck_hours"),
				RetryMaxAttempts:    v.GetInt("library.scanner.retry_max_attempts"),
				RetryInitialSeconds: v.GetInt("library.scanner.retry_initial_seconds"),
				RetryMaxSeconds:     v.GetInt("library.scanner.retry_max_seconds"),
			},
		},
		Classification: ClassificationConfig{
			Enabled:            v.GetBool("classification.enabled"),
			AnalyzeOnUpload:    v.GetBool("classification.analyze_on_upload"),
			AutoThreshold:      v.GetFloat64("classification.auto_threshold"),
			ReviewMargin:       v.GetFloat64("classification.review_margin"),
			CalmFlowWeight:     v.GetFloat64("classification.weights.calm_flow"),
			KineticPulseWeight: v.GetFloat64("classification.weights.kinetic_pulse"),
			CosmicDriftWeight:  v.GetFloat64("classification.weights.cosmic_drift"),
			BassImpactWeight:   v.GetFloat64("classification.weights.bass_impact"),
			Analyzer: AnalyzerConfig{
				Mode:                strings.ToLower(strings.TrimSpace(v.GetString("classification.analyzer.mode"))),
				Endpoint:            strings.TrimSpace(v.GetString("classification.analyzer.endpoint")),
				Token:               analyzerToken,
				ID:                  strings.TrimSpace(v.GetString("classification.analyzer.id")),
				Version:             strings.TrimSpace(v.GetString("classification.analyzer.version")),
				ModelVersion:        strings.TrimSpace(v.GetString("classification.analyzer.model_version")),
				TimeoutSeconds:      v.GetInt("classification.analyzer.timeout_seconds"),
				Concurrency:         v.GetInt("classification.analyzer.concurrency"),
				QueueLimit:          v.GetInt("classification.analyzer.queue_limit"),
				MaxFileSizeMB:       v.GetInt("classification.analyzer.max_file_size_mb"),
				MaxDurationSeconds:  v.GetInt("classification.analyzer.max_duration_seconds"),
				RetryMaxAttempts:    v.GetInt("classification.analyzer.retry_max_attempts"),
				RetryInitialSeconds: v.GetInt("classification.analyzer.retry_initial_seconds"),
				RetryMaxSeconds:     v.GetInt("classification.analyzer.retry_max_seconds"),
			},
		},
		Integrations: IntegrationsConfig{
			MusicBee: MusicBeeConfig{
				SubmitToken:    musicBeeSubmitToken,
				SubmitUsername: strings.TrimSpace(v.GetString("integrations.musicbee.submit_username")),
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:                 v.GetBool("rate_limit.enabled"),
			GlobalRequestsPerSecond: v.GetFloat64("rate_limit.global_requests_per_second"),
			GlobalBurst:             v.GetInt("rate_limit.global_burst"),
			AuthRequestsPerSecond:   v.GetFloat64("rate_limit.auth_requests_per_second"),
			AuthBurst:               v.GetInt("rate_limit.auth_burst"),
		},
		Logging: LoggingConfig{
			Level:      strings.ToLower(strings.TrimSpace(v.GetString("logging.level"))),
			AccessLog:  v.GetBool("logging.access_log"),
			MaxSizeMB:  v.GetInt("logging.max_size_mb"),
			MaxBackups: v.GetInt("logging.max_backups"),
			MaxAgeDays: v.GetInt("logging.max_age_days"),
			Compress:   v.GetBool("logging.compress"),
			LocalTime:  v.GetBool("logging.local_time"),
		},
	}
	// Publish only a fully validated snapshot. Callers can never observe a
	// half-populated or invalid AppConfig after a failed reload.
	if err := Validate(loaded); err != nil {
		return err
	}
	AppConfig = loaded
	return nil
}

func setViperDefaults(v *viper.Viper, defaults Config) {
	v.SetDefault("server.listen_address", defaults.Server.ListenAddress)
	v.SetDefault("server.port", defaults.Server.Port)
	v.SetDefault("server.mode", defaults.Server.Mode)
	v.SetDefault("server.read_header_timeout", defaults.Server.ReadHeaderTimeout)
	v.SetDefault("server.read_timeout", defaults.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", defaults.Server.WriteTimeout)
	v.SetDefault("server.idle_timeout", defaults.Server.IdleTimeout)
	v.SetDefault("server.shutdown_timeout", defaults.Server.ShutdownTimeout)
	v.SetDefault("server.readiness_timeout", defaults.Server.ReadinessTimeout)
	v.SetDefault("server.upload_dir", defaults.Server.UploadDir)
	v.SetDefault("server.log_file", defaults.Server.LogFile)
	v.SetDefault("server.max_json_body_size_mb", defaults.Server.MaxJSONBodySizeMB)
	v.SetDefault("server.max_audio_size_mb", defaults.Server.MaxAudioSizeMB)
	v.SetDefault("server.max_cover_size_mb", defaults.Server.MaxCoverSizeMB)
	v.SetDefault("server.allowed_origins", defaults.Server.AllowedOrigins)
	v.SetDefault("server.trusted_proxies", defaults.Server.TrustedProxies)
	v.SetDefault("database.type", defaults.Database.Type)
	v.SetDefault("database.path", defaults.Database.Path)
	v.SetDefault("database.port", defaults.Database.Port)
	v.SetDefault("database.sslmode", defaults.Database.SSLMode)
	v.SetDefault("database.log_level", defaults.Database.LogLevel)
	v.SetDefault("database.connect_timeout_seconds", defaults.Database.ConnectTimeoutSeconds)
	v.SetDefault("database.max_open_connections", defaults.Database.MaxOpenConnections)
	v.SetDefault("database.max_idle_connections", defaults.Database.MaxIdleConnections)
	v.SetDefault("database.connection_max_lifetime_minutes", defaults.Database.ConnectionMaxLifetimeMinutes)
	v.SetDefault("database.connection_max_idle_time_minutes", defaults.Database.ConnectionMaxIdleTimeMinutes)
	v.SetDefault("jwt.access_token_ttl_minutes", defaults.JWT.AccessTokenTTLMinutes)
	v.SetDefault("jwt.refresh_token_ttl_days", defaults.JWT.RefreshTokenTTLDays)
	v.SetDefault("jwt.refresh_cookie_secure", defaults.JWT.RefreshCookieSecure)
	v.SetDefault("metrics.enabled", defaults.Metrics.Enabled)
	v.SetDefault("metrics.token", defaults.Metrics.Token)
	v.SetDefault("admin.bootstrap.enabled", defaults.AdminBootstrap.Enabled)
	v.SetDefault("admin.bootstrap.full_name", defaults.AdminBootstrap.FullName)
	v.SetDefault("admin.bootstrap.reset_password", defaults.AdminBootstrap.ResetPassword)
	v.SetDefault("access.library_mode", defaults.Access.LibraryMode)
	v.SetDefault("access.registration_mode", defaults.Access.RegistrationMode)
	v.SetDefault("access.media_url_ttl_minutes", defaults.Access.MediaURLTTLMinutes)
	v.SetDefault("library.health_check_interval_seconds", defaults.Library.HealthCheckIntervalSeconds)
	v.SetDefault("library.scanner.enabled", defaults.Library.Scanner.Enabled)
	v.SetDefault("library.scanner.max_file_size_mb", defaults.Library.Scanner.MaxFileSizeMB)
	v.SetDefault("library.scanner.max_tag_size_mb", defaults.Library.Scanner.MaxTagSizeMB)
	v.SetDefault("library.scanner.min_file_age_seconds", defaults.Library.Scanner.MinFileAgeSeconds)
	v.SetDefault("library.scanner.hash_recheck_hours", defaults.Library.Scanner.HashRecheckHours)
	v.SetDefault("library.scanner.retry_max_attempts", defaults.Library.Scanner.RetryMaxAttempts)
	v.SetDefault("library.scanner.retry_initial_seconds", defaults.Library.Scanner.RetryInitialSeconds)
	v.SetDefault("library.scanner.retry_max_seconds", defaults.Library.Scanner.RetryMaxSeconds)
	v.SetDefault("classification.enabled", defaults.Classification.Enabled)
	v.SetDefault("classification.analyze_on_upload", defaults.Classification.AnalyzeOnUpload)
	v.SetDefault("classification.auto_threshold", defaults.Classification.AutoThreshold)
	v.SetDefault("classification.review_margin", defaults.Classification.ReviewMargin)
	v.SetDefault("classification.weights.calm_flow", defaults.Classification.CalmFlowWeight)
	v.SetDefault("classification.weights.kinetic_pulse", defaults.Classification.KineticPulseWeight)
	v.SetDefault("classification.weights.cosmic_drift", defaults.Classification.CosmicDriftWeight)
	v.SetDefault("classification.weights.bass_impact", defaults.Classification.BassImpactWeight)
	v.SetDefault("classification.analyzer.mode", defaults.Classification.Analyzer.Mode)
	v.SetDefault("classification.analyzer.endpoint", defaults.Classification.Analyzer.Endpoint)
	v.SetDefault("classification.analyzer.id", defaults.Classification.Analyzer.ID)
	v.SetDefault("classification.analyzer.version", defaults.Classification.Analyzer.Version)
	v.SetDefault("classification.analyzer.model_version", defaults.Classification.Analyzer.ModelVersion)
	v.SetDefault("classification.analyzer.timeout_seconds", defaults.Classification.Analyzer.TimeoutSeconds)
	v.SetDefault("classification.analyzer.concurrency", defaults.Classification.Analyzer.Concurrency)
	v.SetDefault("classification.analyzer.queue_limit", defaults.Classification.Analyzer.QueueLimit)
	v.SetDefault("classification.analyzer.max_file_size_mb", defaults.Classification.Analyzer.MaxFileSizeMB)
	v.SetDefault("classification.analyzer.max_duration_seconds", defaults.Classification.Analyzer.MaxDurationSeconds)
	v.SetDefault("classification.analyzer.retry_max_attempts", defaults.Classification.Analyzer.RetryMaxAttempts)
	v.SetDefault("classification.analyzer.retry_initial_seconds", defaults.Classification.Analyzer.RetryInitialSeconds)
	v.SetDefault("classification.analyzer.retry_max_seconds", defaults.Classification.Analyzer.RetryMaxSeconds)
	v.SetDefault("integrations.musicbee.submit_token", defaults.Integrations.MusicBee.SubmitToken)
	v.SetDefault("integrations.musicbee.submit_username", defaults.Integrations.MusicBee.SubmitUsername)
	v.SetDefault("rate_limit.enabled", defaults.RateLimit.Enabled)
	v.SetDefault("rate_limit.global_requests_per_second", defaults.RateLimit.GlobalRequestsPerSecond)
	v.SetDefault("rate_limit.global_burst", defaults.RateLimit.GlobalBurst)
	v.SetDefault("rate_limit.auth_requests_per_second", defaults.RateLimit.AuthRequestsPerSecond)
	v.SetDefault("rate_limit.auth_burst", defaults.RateLimit.AuthBurst)
	v.SetDefault("logging.max_size_mb", defaults.Logging.MaxSizeMB)
	v.SetDefault("logging.level", defaults.Logging.Level)
	v.SetDefault("logging.access_log", defaults.Logging.AccessLog)
	v.SetDefault("logging.max_backups", defaults.Logging.MaxBackups)
	v.SetDefault("logging.max_age_days", defaults.Logging.MaxAgeDays)
	v.SetDefault("logging.compress", defaults.Logging.Compress)
	v.SetDefault("logging.local_time", defaults.Logging.LocalTime)
}

const maxSecretFileBytes = 64 << 10

// secretValue supports the conventional ENV or ENV_FILE forms. A file value
// has environment-level priority over YAML but cannot be combined with a
// directly supplied environment value, which keeps secret provenance explicit.
func secretValue(configKey, envName string) (string, error) {
	return secretValueFrom(viper.GetViper(), configKey, envName)
}

func secretValueFrom(v *viper.Viper, configKey, envName string) (string, error) {
	directValue := os.Getenv(envName)
	filePath := strings.TrimSpace(os.Getenv(envName + "_FILE"))
	if directValue != "" && filePath != "" {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", envName, envName)
	}
	if filePath == "" {
		return v.GetString(configKey), nil
	}

	// #nosec G304,G703 -- this path is an explicit administrator-controlled *_FILE setting.
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", envName, err)
	}

	value, readErr := io.ReadAll(io.LimitReader(file, maxSecretFileBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return "", fmt.Errorf("read %s_FILE: %w", envName, readErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close %s_FILE: %w", envName, closeErr)
	}
	if len(value) > maxSecretFileBytes {
		return "", fmt.Errorf("%s_FILE exceeds %d bytes", envName, maxSecretFileBytes)
	}
	return strings.TrimRight(string(value), "\r\n"), nil
}

func NormalizeAllowedOrigins(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	origins := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			origin := strings.TrimSpace(item)
			if origin == "" {
				continue
			}
			parsed, err := url.Parse(origin)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
				parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
				return nil, fmt.Errorf("invalid allowed origin %q: expected http(s)://host[:port]", origin)
			}
			normalized := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			origins = append(origins, normalized)
		}
	}
	return origins, nil
}

func ValidateMetricsConfig(cfg MetricsConfig) error {
	if cfg.Enabled && strings.TrimSpace(cfg.Token) == "" {
		return errors.New("metrics token is required when metrics are enabled")
	}
	return nil
}

func ValidateAdminBootstrapConfig(cfg AdminBootstrapConfig) error {
	if !cfg.Enabled {
		return nil
	}
	usernameLength := utf8.RuneCountInString(cfg.Username)
	if usernameLength < 3 || usernameLength > 50 {
		return errors.New("enabled admin.bootstrap requires a username between 3 and 50 characters")
	}
	if len(cfg.Email) > 255 {
		return errors.New("admin.bootstrap.email exceeds 255 bytes")
	}
	address, err := mail.ParseAddress(cfg.Email)
	if err != nil || address.Address != cfg.Email {
		return errors.New("enabled admin.bootstrap requires a valid email address")
	}
	if utf8.RuneCountInString(cfg.FullName) > 255 {
		return errors.New("admin.bootstrap.full_name exceeds 255 characters")
	}
	if err := password.ValidateNewPassword(cfg.Password); err != nil {
		return fmt.Errorf("admin.bootstrap.password: %w", err)
	}
	return nil
}

func NormalizeTrustedProxies(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	proxies := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			candidate := strings.TrimSpace(item)
			if candidate == "" {
				continue
			}

			normalized := ""
			if ip := net.ParseIP(candidate); ip != nil {
				normalized = ip.String()
			} else if _, network, err := net.ParseCIDR(candidate); err == nil {
				normalized = network.String()
			} else {
				return nil, fmt.Errorf("invalid trusted proxy %q: expected an IP address or CIDR", candidate)
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			proxies = append(proxies, normalized)
		}
	}
	return proxies, nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if err := ValidateServerConfig(cfg.Server); err != nil {
		return err
	}
	if err := ValidateDatabaseConfig(cfg.Database); err != nil {
		return err
	}
	if err := ValidateJWTSecret(cfg.JWT.Secret, cfg.Server.Mode); err != nil {
		return err
	}
	if cfg.JWT.AccessTokenTTLMinutes <= 0 {
		return errors.New("jwt.access_token_ttl_minutes must be greater than zero")
	}
	if cfg.JWT.RefreshTokenTTLDays <= 0 {
		return errors.New("jwt.refresh_token_ttl_days must be greater than zero")
	}
	if err := ValidateMetricsConfig(cfg.Metrics); err != nil {
		return err
	}
	if err := ValidateAdminBootstrapConfig(cfg.AdminBootstrap); err != nil {
		return err
	}
	if err := ValidateAccessConfig(cfg.Access); err != nil {
		return err
	}
	if err := ValidateLibraryConfig(cfg.Library); err != nil {
		return err
	}
	if err := ValidateClassificationConfig(cfg.Classification); err != nil {
		return err
	}
	if err := ValidateMusicBeeConfig(cfg.Integrations.MusicBee); err != nil {
		return err
	}
	if err := ValidateRateLimitConfig(cfg.RateLimit); err != nil {
		return err
	}
	return ValidateLoggingConfig(cfg.Logging)
}

func ValidateClassificationConfig(cfg ClassificationConfig) error {
	if !cfg.Enabled {
		if cfg.AnalyzeOnUpload {
			return errors.New("classification.analyze_on_upload requires classification.enabled")
		}
		return nil
	}
	if cfg.AutoThreshold <= 0 || cfg.AutoThreshold > 1 {
		return errors.New("classification.auto_threshold must be greater than 0 and at most 1")
	}
	if cfg.ReviewMargin < 0 || cfg.ReviewMargin > 1 {
		return errors.New("classification.review_margin must be between 0 and 1")
	}
	for name, value := range map[string]float64{
		"classification.weights.calm_flow":     cfg.CalmFlowWeight,
		"classification.weights.kinetic_pulse": cfg.KineticPulseWeight,
		"classification.weights.cosmic_drift":  cfg.CosmicDriftWeight,
		"classification.weights.bass_impact":   cfg.BassImpactWeight,
	} {
		if value <= 0 || value > 2 {
			return fmt.Errorf("%s must be greater than 0 and at most 2", name)
		}
	}
	if cfg.AnalyzeOnUpload && cfg.Analyzer.Mode != "http" {
		return errors.New("classification.analyze_on_upload requires classification.analyzer.mode=http")
	}
	return ValidateAnalyzerConfig(cfg.Analyzer)
}

func ValidateAnalyzerConfig(cfg AnalyzerConfig) error {
	switch cfg.Mode {
	case "", "disabled":
		return nil
	case "http":
	default:
		return fmt.Errorf("classification.analyzer.mode must be disabled or http, got %q", cfg.Mode)
	}
	endpoint, err := url.Parse(cfg.Endpoint)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
		endpoint.User != nil || endpoint.Fragment != "" {
		return errors.New("classification.analyzer.endpoint must be an absolute http(s) URL without credentials or fragment")
	}
	if len([]byte(cfg.Token)) < 32 {
		return errors.New("classification.analyzer.token must contain at least 32 bytes in http mode")
	}
	for name, value := range map[string]string{
		"classification.analyzer.id":            cfg.ID,
		"classification.analyzer.version":       cfg.Version,
		"classification.analyzer.model_version": cfg.ModelVersion,
	} {
		if value == "" || len(value) > 100 {
			return fmt.Errorf("%s must contain 1 to 100 characters in http mode", name)
		}
	}
	if cfg.TimeoutSeconds <= 0 || cfg.TimeoutSeconds > 3600 {
		return errors.New("classification.analyzer.timeout_seconds must be between 1 and 3600")
	}
	if cfg.Concurrency <= 0 || cfg.Concurrency > 8 {
		return errors.New("classification.analyzer.concurrency must be between 1 and 8")
	}
	if cfg.QueueLimit <= 0 || cfg.QueueLimit > 100000 {
		return errors.New("classification.analyzer.queue_limit must be between 1 and 100000")
	}
	if cfg.MaxFileSizeMB <= 0 || cfg.MaxDurationSeconds <= 0 {
		return errors.New("classification analyzer file-size and duration limits must be greater than zero")
	}
	if err := validateMegabytesFits("classification.analyzer.max_file_size_mb", cfg.MaxFileSizeMB); err != nil {
		return err
	}
	if err := validateDurationFits("classification.analyzer.max_duration_seconds", cfg.MaxDurationSeconds, time.Second); err != nil {
		return err
	}
	if cfg.RetryMaxAttempts <= 0 || cfg.RetryMaxAttempts > 20 {
		return errors.New("classification.analyzer.retry_max_attempts must be between 1 and 20")
	}
	if cfg.RetryInitialSeconds <= 0 || cfg.RetryMaxSeconds < cfg.RetryInitialSeconds {
		return errors.New("classification analyzer retry delays must be positive and max must not be smaller than initial")
	}
	if err := validateDurationFits("classification.analyzer.timeout_seconds", cfg.TimeoutSeconds, time.Second); err != nil {
		return err
	}
	if err := validateDurationFits("classification.analyzer.retry_max_seconds", cfg.RetryMaxSeconds, time.Second); err != nil {
		return err
	}
	return nil
}

func ValidateLibraryConfig(cfg LibraryConfig) error {
	if cfg.HealthCheckIntervalSeconds < 0 {
		return errors.New("library.health_check_interval_seconds cannot be negative")
	}
	if err := validateDurationFits("library.health_check_interval_seconds", cfg.HealthCheckIntervalSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.MaxFileSizeMB <= 0 {
		return errors.New("library.scanner.max_file_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("library.scanner.max_file_size_mb", cfg.Scanner.MaxFileSizeMB); err != nil {
		return err
	}
	if cfg.Scanner.MaxTagSizeMB <= 0 || cfg.Scanner.MaxTagSizeMB > 64 {
		return errors.New("library.scanner.max_tag_size_mb must be between 1 and 64")
	}
	if cfg.Scanner.MinFileAgeSeconds < 0 {
		return errors.New("library.scanner.min_file_age_seconds cannot be negative")
	}
	if err := validateDurationFits("library.scanner.min_file_age_seconds", cfg.Scanner.MinFileAgeSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.HashRecheckHours < 0 {
		return errors.New("library.scanner.hash_recheck_hours cannot be negative")
	}
	if err := validateDurationFits("library.scanner.hash_recheck_hours", cfg.Scanner.HashRecheckHours, time.Hour); err != nil {
		return err
	}
	if cfg.Scanner.RetryMaxAttempts <= 0 {
		return errors.New("library.scanner.retry_max_attempts must be greater than zero")
	}
	if cfg.Scanner.RetryInitialSeconds <= 0 {
		return errors.New("library.scanner.retry_initial_seconds must be greater than zero")
	}
	if err := validateDurationFits("library.scanner.retry_initial_seconds", cfg.Scanner.RetryInitialSeconds, time.Second); err != nil {
		return err
	}
	if cfg.Scanner.RetryMaxSeconds < cfg.Scanner.RetryInitialSeconds {
		return errors.New("library.scanner.retry_max_seconds must be greater than or equal to retry_initial_seconds")
	}
	if err := validateDurationFits("library.scanner.retry_max_seconds", cfg.Scanner.RetryMaxSeconds, time.Second); err != nil {
		return err
	}
	return nil
}

func ValidateAccessConfig(cfg AccessConfig) error {
	switch cfg.LibraryMode {
	case LibraryAccessPublic, LibraryAccessAuthenticated:
	default:
		return fmt.Errorf("access.library_mode must be public or authenticated, got %q", cfg.LibraryMode)
	}
	switch cfg.RegistrationMode {
	case RegistrationOpen, RegistrationAdmin:
	default:
		return fmt.Errorf("access.registration_mode must be open or admin, got %q", cfg.RegistrationMode)
	}
	if cfg.MediaURLTTLMinutes <= 0 {
		return errors.New("access.media_url_ttl_minutes must be greater than zero")
	}
	if err := validateDurationFits("access.media_url_ttl_minutes", cfg.MediaURLTTLMinutes, time.Minute); err != nil {
		return err
	}
	return nil
}

func ValidateMusicBeeConfig(cfg MusicBeeConfig) error {
	hasToken := strings.TrimSpace(cfg.SubmitToken) != ""
	hasUsername := strings.TrimSpace(cfg.SubmitUsername) != ""
	if hasToken != hasUsername {
		return errors.New("integrations.musicbee.submit_token and submit_username must be configured together")
	}
	if hasToken && len([]byte(cfg.SubmitToken)) < 32 {
		return errors.New("integrations.musicbee.submit_token must contain at least 32 bytes")
	}
	return nil
}

func ValidateServerConfig(cfg ServerConfig) error {
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("server.port must be an integer between 1 and 65535, got %q", cfg.Port)
	}
	switch cfg.Mode {
	case "debug", "release", "test":
	default:
		return fmt.Errorf("server.mode must be debug, release, or test, got %q", cfg.Mode)
	}
	for name, value := range map[string]int{
		"server.read_header_timeout": cfg.ReadHeaderTimeout,
		"server.read_timeout":        cfg.ReadTimeout,
		"server.write_timeout":       cfg.WriteTimeout,
		"server.idle_timeout":        cfg.IdleTimeout,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
		if err := validateDurationFits(name, value, time.Second); err != nil {
			return err
		}
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("server.shutdown_timeout must be greater than zero")
	}
	if err := validateDurationFits("server.shutdown_timeout", cfg.ShutdownTimeout, time.Second); err != nil {
		return err
	}
	if cfg.ReadinessTimeout <= 0 {
		return errors.New("server.readiness_timeout must be greater than zero")
	}
	if err := validateDurationFits("server.readiness_timeout", cfg.ReadinessTimeout, time.Second); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.UploadDir) == "" {
		return errors.New("server.upload_dir cannot be empty")
	}
	if cfg.MaxJSONBodySizeMB <= 0 {
		return errors.New("server.max_json_body_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_json_body_size_mb", cfg.MaxJSONBodySizeMB); err != nil {
		return err
	}
	if cfg.MaxAudioSizeMB <= 0 {
		return errors.New("server.max_audio_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_audio_size_mb", cfg.MaxAudioSizeMB); err != nil {
		return err
	}
	if cfg.MaxCoverSizeMB <= 0 {
		return errors.New("server.max_cover_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("server.max_cover_size_mb", cfg.MaxCoverSizeMB); err != nil {
		return err
	}
	audioBytes := int64(cfg.MaxAudioSizeMB) * (1 << 20)
	coverBytes := int64(cfg.MaxCoverSizeMB) * (1 << 20)
	if audioBytes > maxSignedInt64-coverBytes-(1<<20) {
		return errors.New("server audio and cover limits are too large to form a safe multipart request limit")
	}
	return nil
}

func ValidateDatabaseConfig(cfg DatabaseConfig) error {
	switch cfg.LogLevel {
	case "auto", "silent", "error", "warn", "info":
	default:
		return fmt.Errorf("database.log_level must be auto, silent, error, warn, or info, got %q", cfg.LogLevel)
	}
	switch cfg.Type {
	case "sqlite":
		if strings.TrimSpace(cfg.Path) == "" {
			return errors.New("database.path is required for sqlite")
		}
	case "postgres":
		missing := make([]string, 0, 3)
		if strings.TrimSpace(cfg.Host) == "" {
			missing = append(missing, "database.host")
		}
		if strings.TrimSpace(cfg.User) == "" {
			missing = append(missing, "database.user")
		}
		if strings.TrimSpace(cfg.Name) == "" {
			missing = append(missing, "database.name")
		}
		if len(missing) > 0 {
			return fmt.Errorf("postgres configuration is incomplete; missing %s", strings.Join(missing, ", "))
		}
		port, err := strconv.Atoi(cfg.Port)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("database.port must be an integer between 1 and 65535, got %q", cfg.Port)
		}
		validSSLModes := map[string]struct{}{
			"disable": {}, "allow": {}, "prefer": {}, "require": {}, "verify-ca": {}, "verify-full": {},
		}
		if _, ok := validSSLModes[cfg.SSLMode]; !ok {
			return fmt.Errorf("database.sslmode has unsupported value %q", cfg.SSLMode)
		}
	default:
		return fmt.Errorf("database.type must be sqlite or postgres, got %q", cfg.Type)
	}
	if cfg.ConnectTimeoutSeconds <= 0 {
		return errors.New("database.connect_timeout_seconds must be greater than zero")
	}
	for name, value := range map[string]int{
		"database.max_open_connections":             cfg.MaxOpenConnections,
		"database.max_idle_connections":             cfg.MaxIdleConnections,
		"database.connection_max_lifetime_minutes":  cfg.ConnectionMaxLifetimeMinutes,
		"database.connection_max_idle_time_minutes": cfg.ConnectionMaxIdleTimeMinutes,
	} {
		if value < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
		if strings.Contains(name, "_minutes") {
			if err := validateDurationFits(name, value, time.Minute); err != nil {
				return err
			}
		}
	}
	if cfg.MaxOpenConnections > 0 && cfg.MaxIdleConnections > cfg.MaxOpenConnections {
		return errors.New("database.max_idle_connections cannot exceed database.max_open_connections")
	}
	return nil
}

func ValidateRateLimitConfig(cfg RateLimitConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if !finitePositive(cfg.GlobalRequestsPerSecond) || cfg.GlobalBurst <= 0 {
		return errors.New("enabled rate_limit requires positive global_requests_per_second and global_burst")
	}
	if !finitePositive(cfg.AuthRequestsPerSecond) || cfg.AuthBurst <= 0 {
		return errors.New("enabled rate_limit requires positive auth_requests_per_second and auth_burst")
	}
	return nil
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsInf(value, 0) && !math.IsNaN(value)
}

func ValidateLoggingConfig(cfg LoggingConfig) error {
	switch cfg.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging.level must be debug, info, warn, or error, got %q", cfg.Level)
	}
	if cfg.MaxSizeMB <= 0 {
		return errors.New("logging.max_size_mb must be greater than zero")
	}
	if err := validateMegabytesFits("logging.max_size_mb", cfg.MaxSizeMB); err != nil {
		return err
	}
	if cfg.MaxBackups < 0 {
		return errors.New("logging.max_backups cannot be negative")
	}
	if cfg.MaxAgeDays < 0 {
		return errors.New("logging.max_age_days cannot be negative")
	}
	return nil
}

var maxSignedInt64 = int64(^uint64(0) >> 1)

func validateDurationFits(name string, value int, unit time.Duration) error {
	if value > 0 && int64(value) > maxSignedInt64/int64(unit) {
		return fmt.Errorf("%s is too large", name)
	}
	return nil
}

func validateMegabytesFits(name string, value int) error {
	if value > 0 && int64(value) > maxSignedInt64/(1<<20) {
		return fmt.Errorf("%s is too large", name)
	}
	return nil
}

// ValidateJWTSecret checks that the JWT secret is strong enough for non-debug modes.
// Debug mode may use the documented development placeholder, but never an empty
// signing key. Non-debug modes require an explicitly configured secret.
func ValidateJWTSecret(secret, mode string) error {
	if strings.TrimSpace(secret) == "" {
		return errors.New("jwt.secret is required")
	}
	if len([]byte(secret)) < 32 {
		return errors.New("jwt.secret must contain at least 32 bytes")
	}
	if mode != "debug" && secret == "your-secret-key-change-in-production" {
		return errors.New("weak JWT secret rejected: must be set to a strong random value in non-debug mode")
	}
	return nil
}

// addSearchPaths 按优先级从高到低添加配置文件搜索路径。
// Viper 只读取第一个找到的配置文件，不会合并后续路径。
func addSearchPaths(v *viper.Viper) {
	// 1. 最高优先级：二进制所在目录
	if exe, err := os.Executable(); err == nil {
		v.AddConfigPath(filepath.Dir(exe))
	}

	// 2. Docker volume 挂载常用路径
	v.AddConfigPath("/data")

	// 3. 系统级配置
	v.AddConfigPath("/etc/music-online")

	// 4. XDG 配置目录
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		v.AddConfigPath(filepath.Join(xdg, "music-online"))
	} else if home := os.Getenv("HOME"); home != "" {
		v.AddConfigPath(filepath.Join(home, ".config", "music-online"))
	}

	// 5. 开发：从 cmd/server 运行时往回找
	v.AddConfigPath("..")
	v.AddConfigPath("../..")

	// 6. 最低优先级：当前目录和子目录 config/
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
}
