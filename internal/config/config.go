// Package config config.go - 配置管理
// 加载 YAML 配置文件，支持环境变量和命令行参数覆盖
package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	pklog "github.com/kyangconn/music-online-go/internal/pkg/log"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Port         string
	Mode         string
	ReadTimeout  int
	WriteTimeout int
	UploadDir    string
	LogFile      string
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

type SecurityConfig struct {
	PasswordSalt string
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
	viper.SetDefault("server.port", "3060")
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.upload_dir", "uploads")
	viper.SetDefault("server.log_file", "")
	viper.SetDefault("database.type", "postgres")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("jwt.expire_hour", 24)

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

	AppConfig = &Config{
		Server: ServerConfig{
			Port:         viper.GetString("server.port"),
			Mode:         viper.GetString("server.mode"),
			ReadTimeout:  viper.GetInt("server.read_timeout"),
			WriteTimeout: viper.GetInt("server.write_timeout"),
			UploadDir:    viper.GetString("server.upload_dir"),
			LogFile:      logFile,
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
	}
	pklog.Infof("Loaded config file: %s", viper.ConfigFileUsed())

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
