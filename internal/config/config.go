// Package config config.go - 配置管理
// 加载 YAML 配置文件，支持环境变量和命令行参数覆盖
package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	SourceFile     string
	Server         ServerConfig
	Database       DatabaseConfig
	JWT            JWTConfig
	Metrics        MetricsConfig
	AdminBootstrap AdminBootstrapConfig
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
	Secret     string
	ExpireHour int
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

func LoadConfig() error {
	viper.Reset()
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Viper 读取第一个找到的文件，因此按优先级从高到低添加搜索路径。
	addSearchPaths()

	// 环境变量覆盖: MO_CONFIG_FILE
	if envPath := os.Getenv("MO_CONFIG_FILE"); envPath != "" {
		viper.SetConfigFile(envPath)
	}

	// 设置默认值
	viper.SetDefault("server.listen_address", "")
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_header_timeout", 10)
	viper.SetDefault("server.read_timeout", 0)
	viper.SetDefault("server.write_timeout", 0)
	viper.SetDefault("server.idle_timeout", 60)
	viper.SetDefault("server.shutdown_timeout", 15)
	viper.SetDefault("server.readiness_timeout", 2)
	viper.SetDefault("server.upload_dir", "uploads")
	viper.SetDefault("server.log_file", "")
	viper.SetDefault("server.max_json_body_size_mb", 1)
	viper.SetDefault("server.max_audio_size_mb", 200)
	viper.SetDefault("server.max_cover_size_mb", 10)
	viper.SetDefault("server.allowed_origins", []string{})
	viper.SetDefault("server.trusted_proxies", []string{})
	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.path", "music.db")
	viper.SetDefault("database.port", "5432")
	viper.SetDefault("database.sslmode", "prefer")
	viper.SetDefault("database.log_level", "auto")
	viper.SetDefault("database.connect_timeout_seconds", 10)
	viper.SetDefault("database.max_open_connections", 0)
	viper.SetDefault("database.max_idle_connections", 0)
	viper.SetDefault("database.connection_max_lifetime_minutes", 60)
	viper.SetDefault("database.connection_max_idle_time_minutes", 10)
	viper.SetDefault("jwt.expire_hour", 24)
	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.token", "")
	viper.SetDefault("admin.bootstrap.enabled", false)
	viper.SetDefault("admin.bootstrap.full_name", "Administrator")
	viper.SetDefault("admin.bootstrap.reset_password", false)
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.global_requests_per_second", 20.0)
	viper.SetDefault("rate_limit.global_burst", 50)
	viper.SetDefault("rate_limit.auth_requests_per_second", 1.0)
	viper.SetDefault("rate_limit.auth_burst", 5)
	viper.SetDefault("logging.max_size_mb", 50)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.access_log", true)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age_days", 28)
	viper.SetDefault("logging.compress", true)
	viper.SetDefault("logging.local_time", true)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			return fmt.Errorf("read config file: %w", err)
		}
	}

	logFile := os.Getenv("MO_LOG_FILE")
	if logFile == "" {
		logFile = viper.GetString("server.log_file")
	}
	allowedOrigins, err := NormalizeAllowedOrigins(viper.GetStringSlice("server.allowed_origins"))
	if err != nil {
		return err
	}
	trustedProxies, err := NormalizeTrustedProxies(viper.GetStringSlice("server.trusted_proxies"))
	if err != nil {
		return err
	}
	databasePassword, err := secretValue("database.password", "DATABASE_PASSWORD")
	if err != nil {
		return err
	}
	jwtSecret, err := secretValue("jwt.secret", "JWT_SECRET")
	if err != nil {
		return err
	}
	metricsToken, err := secretValue("metrics.token", "METRICS_TOKEN")
	if err != nil {
		return err
	}
	bootstrapPassword, err := secretValue("admin.bootstrap.password", "ADMIN_BOOTSTRAP_PASSWORD")
	if err != nil {
		return err
	}

	AppConfig = &Config{
		SourceFile: viper.ConfigFileUsed(),
		Server: ServerConfig{
			ListenAddress:     viper.GetString("server.listen_address"),
			Port:              viper.GetString("server.port"),
			Mode:              strings.ToLower(strings.TrimSpace(viper.GetString("server.mode"))),
			ReadHeaderTimeout: viper.GetInt("server.read_header_timeout"),
			ReadTimeout:       viper.GetInt("server.read_timeout"),
			WriteTimeout:      viper.GetInt("server.write_timeout"),
			IdleTimeout:       viper.GetInt("server.idle_timeout"),
			ShutdownTimeout:   viper.GetInt("server.shutdown_timeout"),
			ReadinessTimeout:  viper.GetInt("server.readiness_timeout"),
			UploadDir:         viper.GetString("server.upload_dir"),
			LogFile:           logFile,
			MaxJSONBodySizeMB: viper.GetInt("server.max_json_body_size_mb"),
			MaxAudioSizeMB:    viper.GetInt("server.max_audio_size_mb"),
			MaxCoverSizeMB:    viper.GetInt("server.max_cover_size_mb"),
			AllowedOrigins:    allowedOrigins,
			TrustedProxies:    trustedProxies,
		},
		Database: DatabaseConfig{
			Type:                         strings.ToLower(strings.TrimSpace(viper.GetString("database.type"))),
			Host:                         viper.GetString("database.host"),
			Port:                         strings.TrimSpace(viper.GetString("database.port")),
			User:                         viper.GetString("database.user"),
			Password:                     databasePassword,
			Name:                         viper.GetString("database.name"),
			SSLMode:                      strings.ToLower(strings.TrimSpace(viper.GetString("database.sslmode"))),
			LogLevel:                     strings.ToLower(strings.TrimSpace(viper.GetString("database.log_level"))),
			Path:                         viper.GetString("database.path"),
			ConnectTimeoutSeconds:        viper.GetInt("database.connect_timeout_seconds"),
			MaxOpenConnections:           viper.GetInt("database.max_open_connections"),
			MaxIdleConnections:           viper.GetInt("database.max_idle_connections"),
			ConnectionMaxLifetimeMinutes: viper.GetInt("database.connection_max_lifetime_minutes"),
			ConnectionMaxIdleTimeMinutes: viper.GetInt("database.connection_max_idle_time_minutes"),
		},
		JWT: JWTConfig{
			Secret:     jwtSecret,
			ExpireHour: viper.GetInt("jwt.expire_hour"),
		},
		Metrics: MetricsConfig{
			Enabled: viper.GetBool("metrics.enabled"),
			Token:   metricsToken,
		},
		AdminBootstrap: AdminBootstrapConfig{
			Enabled:       viper.GetBool("admin.bootstrap.enabled"),
			Username:      viper.GetString("admin.bootstrap.username"),
			Email:         viper.GetString("admin.bootstrap.email"),
			Password:      bootstrapPassword,
			FullName:      viper.GetString("admin.bootstrap.full_name"),
			ResetPassword: viper.GetBool("admin.bootstrap.reset_password"),
		},
		RateLimit: RateLimitConfig{
			Enabled:                 viper.GetBool("rate_limit.enabled"),
			GlobalRequestsPerSecond: viper.GetFloat64("rate_limit.global_requests_per_second"),
			GlobalBurst:             viper.GetInt("rate_limit.global_burst"),
			AuthRequestsPerSecond:   viper.GetFloat64("rate_limit.auth_requests_per_second"),
			AuthBurst:               viper.GetInt("rate_limit.auth_burst"),
		},
		Logging: LoggingConfig{
			Level:      strings.ToLower(strings.TrimSpace(viper.GetString("logging.level"))),
			AccessLog:  viper.GetBool("logging.access_log"),
			MaxSizeMB:  viper.GetInt("logging.max_size_mb"),
			MaxBackups: viper.GetInt("logging.max_backups"),
			MaxAgeDays: viper.GetInt("logging.max_age_days"),
			Compress:   viper.GetBool("logging.compress"),
			LocalTime:  viper.GetBool("logging.local_time"),
		},
	}
	return Validate(AppConfig)
}

const maxSecretFileBytes = 64 << 10

// secretValue supports the conventional ENV or ENV_FILE forms. A file value
// has environment-level priority over YAML but cannot be combined with a
// directly supplied environment value, which keeps secret provenance explicit.
func secretValue(configKey, envName string) (string, error) {
	directValue := os.Getenv(envName)
	filePath := strings.TrimSpace(os.Getenv(envName + "_FILE"))
	if directValue != "" && filePath != "" {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", envName, envName)
	}
	if filePath == "" {
		return viper.GetString(configKey), nil
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
	if cfg.JWT.ExpireHour <= 0 {
		return errors.New("jwt.expire_hour must be greater than zero")
	}
	if err := ValidateMetricsConfig(cfg.Metrics); err != nil {
		return err
	}
	if err := ValidateRateLimitConfig(cfg.RateLimit); err != nil {
		return err
	}
	return ValidateLoggingConfig(cfg.Logging)
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
	}
	if cfg.ShutdownTimeout <= 0 {
		return errors.New("server.shutdown_timeout must be greater than zero")
	}
	if cfg.ReadinessTimeout <= 0 {
		return errors.New("server.readiness_timeout must be greater than zero")
	}
	if strings.TrimSpace(cfg.UploadDir) == "" {
		return errors.New("server.upload_dir cannot be empty")
	}
	if cfg.MaxJSONBodySizeMB <= 0 {
		return errors.New("server.max_json_body_size_mb must be greater than zero")
	}
	if cfg.MaxAudioSizeMB <= 0 {
		return errors.New("server.max_audio_size_mb must be greater than zero")
	}
	if cfg.MaxCoverSizeMB <= 0 {
		return errors.New("server.max_cover_size_mb must be greater than zero")
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
	if cfg.GlobalRequestsPerSecond <= 0 || cfg.GlobalBurst <= 0 {
		return errors.New("enabled rate_limit requires positive global_requests_per_second and global_burst")
	}
	if cfg.AuthRequestsPerSecond <= 0 || cfg.AuthBurst <= 0 {
		return errors.New("enabled rate_limit requires positive auth_requests_per_second and auth_burst")
	}
	return nil
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
	if cfg.MaxBackups < 0 {
		return errors.New("logging.max_backups cannot be negative")
	}
	if cfg.MaxAgeDays < 0 {
		return errors.New("logging.max_age_days cannot be negative")
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
func addSearchPaths() {
	// 1. 最高优先级：二进制所在目录
	if exe, err := os.Executable(); err == nil {
		viper.AddConfigPath(filepath.Dir(exe))
	}

	// 2. Docker volume 挂载常用路径
	viper.AddConfigPath("/data")

	// 3. 系统级配置
	viper.AddConfigPath("/etc/music-online")

	// 4. XDG 配置目录
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		viper.AddConfigPath(filepath.Join(xdg, "music-online"))
	} else if home := os.Getenv("HOME"); home != "" {
		viper.AddConfigPath(filepath.Join(home, ".config", "music-online"))
	}

	// 5. 开发：从 cmd/server 运行时往回找
	viper.AddConfigPath("..")
	viper.AddConfigPath("../..")

	// 6. 最低优先级：当前目录和子目录 config/
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
}
