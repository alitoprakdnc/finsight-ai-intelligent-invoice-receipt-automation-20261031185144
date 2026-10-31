package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	once        sync.Once
)

// Init initializes the package‑level logger.
// Pass true for production mode (JSON output, INFO level) or false for development mode (console output, DEBUG level).
func Init(isProd bool) {
	once.Do(func() {
		l, err := newLogger(isProd)
		if err != nil {
			// If logger creation fails, fall back to a no‑op logger to avoid panics.
			globalLogger = zap.NewNop()
			return
		}
		globalLogger = l
	})
}

// L returns the initialized logger. If Init has not been called yet, it lazily creates a development logger.
func L() *zap.Logger {
	if globalLogger == nil {
		// Safe fallback – development logger.
		Init(false)
	}
	return globalLogger
}

// Sync flushes any buffered log entries.
func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// newLogger creates a zap.Logger configured for either production or development environments.
func newLogger(isProd bool) (*zap.Logger, error) {
	var cfg zap.Config
	if isProd {
		cfg = zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
			Development:      false,
			Encoding:         "json",
			EncoderConfig:    productionEncoderConfig(),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			Sampling: &zap.SamplingConfig{
				Initial:    100,
				Thereafter: 100,
			},
		}
	} else {
		cfg = zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Development:      true,
			Encoding:         "console",
			EncoderConfig:    developmentEncoderConfig(),
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		}
	}
	// AddCallerSkip(1) removes the logger wrapper from the call stack, showing the real caller.
	return cfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
}

// productionEncoderConfig defines the JSON encoder used in production.
func productionEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// developmentEncoderConfig defines the console encoder used during development.
func developmentEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// Helper to retrieve environment variable for logger mode; not exported but useful for internal init.
func isProdFromEnv() bool {
	// Common convention: LOG_MODE=production enables prod settings.
	return os.Getenv("LOG_MODE") == "production"
}