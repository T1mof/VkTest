package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger создает и настраивает новый экземпляр Zap logger
func NewLogger(level, format string) (*zap.Logger, error) {
	var config zap.Config

	// Выбор конфигурации в зависимости от формата
	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Настройка уровня логирования
	var logLevel zapcore.Level
	if err := logLevel.UnmarshalText([]byte(level)); err != nil {
		logLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(logLevel)

	// Создание логгера
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
	)
	if err != nil {
		return nil, err
	}

	return logger, nil
}
