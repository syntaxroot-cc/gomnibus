// Package log wraps zap with gomnibus-specific defaults.
package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var global *zap.Logger

func Init(level string) error {
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.Encoding = "console"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	var err error
	global, err = cfg.Build()
	return err
}

func L() *zap.Logger {
	if global == nil {
		global, _ = zap.NewDevelopment()
	}
	return global
}

func Sync() { _ = global.Sync() }
