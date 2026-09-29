package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

func SetupLogger(env string, level string) *zap.Logger {
	if level == "" {
		switch env {
		case EnvLocal, EnvDev:
			level = "debug"
		default:
			level = "info"
		}
	}

	var log *zap.Logger

	switch env {
	case EnvLocal:
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
		applyLevel(&cfg.Level, level)
		log = mustBuild(cfg)
	case EnvDev:
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.OutputPaths = []string{"stdout"}
		applyLevel(&cfg.Level, level)
		log = mustBuild(cfg)
	case EnvProd:
		fallthrough
	default:
		cfg := zap.NewProductionConfig()
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		cfg.OutputPaths = []string{"stdout"}
		applyLevel(&cfg.Level, level)
		log = mustBuild(cfg)
	}

	return log
}

func applyLevel(al *zap.AtomicLevel, level string) {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return
	}
	*al = zap.NewAtomicLevelAt(lvl)
}

func mustBuild(cfg zap.Config) *zap.Logger {
	log, err := cfg.Build(zap.AddCallerSkip(0))
	if err != nil {
		panic(err)
	}
	return log
}
