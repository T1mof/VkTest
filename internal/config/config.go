package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config представляет конфигурацию приложения
type Config struct {
	Server ServerConfig `yaml:"server"`
	Log    LogConfig    `yaml:"log"`
}

// ServerConfig содержит настройки сервера
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// LogConfig содержит настройки логирования
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// LoadConfig загружает конфигурацию из файла и переопределяет её переменными окружения
func LoadConfig(path string) (*Config, error) {
	// Значения по умолчанию
	config := &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 50051,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}

	// Загрузка из файла, если он существует
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, err
			}
		}
	}

	// Переопределение переменными окружения
	if host := os.Getenv("SERVER_HOST"); host != "" {
		config.Server.Host = host
	}
	if portStr := os.Getenv("SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Server.Port = port
		}
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Log.Level = logLevel
	}
	if logFormat := os.Getenv("LOG_FORMAT"); logFormat != "" {
		config.Log.Format = logFormat
	}

	return config, nil
}
