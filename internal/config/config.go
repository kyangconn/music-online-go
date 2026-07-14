// Package config config.go - 配置管理
// 加载 YAML 配置文件，支持环境变量和命令行参数覆盖
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	JWT            JWTConfig
	Metrics        MetricsConfig
	AdminBootstrap AdminBootstrapConfig
}

type ServerConfig struct {
	Port           string
	Mode           string
	ReadTimeout    int
	WriteTimeout   int
	UploadDir      string
	LogFile        string
	MaxAudioSizeMB int
	MaxCoverSizeMB int
	AllowedOrigins []string
}

type DatabaseConfig struct {
	Type     string // postgres / sqlite
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
	Path     string // sqlite 文件路径
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

var AppConfig *Config

func LoadConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 优先级从低到高添加搜索路径
	addSearchPaths()

	// 环境变量覆盖: MO_CONFIG_FILE
	if envPath := os.Getenv("MO_CONFIG_FILE"); envPath != "" {
		viper.SetConfigFile(envPath)
	}

	// 设置默认值
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.upload_dir", "uploads")
	viper.SetDefault("server.log_file", "")
	viper.SetDefault("server.max_audio_size_mb", 200)
	viper.SetDefault("server.max_cover_size_mb", 10)
	viper.SetDefault("server.allowed_origins", []string{})
	viper.SetDefault("database.type", "sqlite")
	viper.SetDefault("database.path", "music.db")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("jwt.expire_hour", 24)
	viper.SetDefault("metrics.enabled", false)
	viper.SetDefault("metrics.token", "")
	viper.SetDefault("admin.bootstrap.enabled", false)
	viper.SetDefault("admin.bootstrap.full_name", "Administrator")
	viper.SetDefault("admin.bootstrap.reset_password", false)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
			pklog.Fatalf("Error reading config file: %v", err)
			return err
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

	AppConfig = &Config{
		Server: ServerConfig{
			Port:           viper.GetString("server.port"),
			Mode:           viper.GetString("server.mode"),
			ReadTimeout:    viper.GetInt("server.read_timeout"),
			WriteTimeout:   viper.GetInt("server.write_timeout"),
			UploadDir:      viper.GetString("server.upload_dir"),
			LogFile:        logFile,
			MaxAudioSizeMB: viper.GetInt("server.max_audio_size_mb"),
			MaxCoverSizeMB: viper.GetInt("server.max_cover_size_mb"),
			AllowedOrigins: allowedOrigins,
		},
		Database: DatabaseConfig{
			Type:     viper.GetString("database.type"),
			Host:     viper.GetString("database.host"),
			Port:     viper.GetString("database.port"),
			User:     viper.GetString("database.user"),
			Password: viper.GetString("database.password"),
			Name:     viper.GetString("database.name"),
			SSLMode:  viper.GetString("database.sslmode"),
			Path:     viper.GetString("database.path"),
		},
		JWT: JWTConfig{
			Secret:     viper.GetString("jwt.secret"),
			ExpireHour: viper.GetInt("jwt.expire_hour"),
		},
		Metrics: MetricsConfig{
			Enabled: viper.GetBool("metrics.enabled"),
			Token:   viper.GetString("metrics.token"),
		},
		AdminBootstrap: AdminBootstrapConfig{
			Enabled:       viper.GetBool("admin.bootstrap.enabled"),
			Username:      viper.GetString("admin.bootstrap.username"),
			Email:         viper.GetString("admin.bootstrap.email"),
			Password:      viper.GetString("admin.bootstrap.password"),
			FullName:      viper.GetString("admin.bootstrap.full_name"),
			ResetPassword: viper.GetBool("admin.bootstrap.reset_password"),
		},
	}
	pklog.Infof("Loaded config file: %s", viper.ConfigFileUsed())

	// P0: validate JWT secret in non-debug modes
	if err := ValidateJWTSecret(AppConfig.JWT.Secret, AppConfig.Server.Mode); err != nil {
		pklog.Fatalf("%v", err)
		return err
	}
	if err := ValidateMetricsConfig(AppConfig.Metrics); err != nil {
		return err
	}

	return nil
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

// ValidateJWTSecret checks that the JWT secret is strong enough for non-debug modes.
// In debug mode any secret (including empty/default) is accepted.
func ValidateJWTSecret(secret, mode string) error {
	if mode != "debug" {
		if secret == "" || secret == "your-secret-key-change-in-production" {
			return errors.New("weak JWT secret rejected: must be set to a strong random value in non-debug mode")
		}
	}
	return nil
}

// addSearchPaths 按优先级从低到高添加配置文件搜索路径
// Viper 的搜索是从前往后，后者覆盖前者
func addSearchPaths() {
	// 4. 开发：当前目录和子目录 config/
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// 3. 开发：从 cmd/server 运行时往回找
	viper.AddConfigPath("..")
	viper.AddConfigPath("../..")

	// 2. XDG 配置目录
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		viper.AddConfigPath(filepath.Join(xdg, "music-online"))
	} else if home := os.Getenv("HOME"); home != "" {
		viper.AddConfigPath(filepath.Join(home, ".config", "music-online"))
	}

	// 2. 系统级配置
	viper.AddConfigPath("/etc/music-online")

	// 2. Docker volume 挂载常用路径
	viper.AddConfigPath("/data")

	// 1. 最高优先级：二进制所在目录
	if exe, err := os.Executable(); err == nil {
		viper.AddConfigPath(filepath.Dir(exe))
	}
}
