package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func SetupLogger(env string) *zap.Logger {
	var log *zap.Logger

	switch env {
	case envLocal:
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig = zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05.000"),
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
		cfg.OutputPaths = []string{"stdout"}
		log = mustBuild(cfg)
	case envDev:
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.OutputPaths = []string{"stdout"}
		log = mustBuild(cfg)
	case envProd:
		fallthrough
	default: // If env config is invalid, set prod settings by default due to security
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.OutputPaths = []string{"stdout"}
		log = mustBuild(cfg)
	}

	return log
}

func mustBuild(cfg zap.Config) *zap.Logger {
	log, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return log
}
