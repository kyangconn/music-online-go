// Package config config_secret.go - 密钥与 *_FILE 读取
package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/viper"
)

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
