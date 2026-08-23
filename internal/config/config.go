// Package config config.go - 配置管理
// 加载 YAML 配置文件，支持环境变量和命令行参数覆盖
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kyangconn/music-online-go/internal/domain"
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
