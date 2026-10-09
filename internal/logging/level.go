package logging

import (
	"go.uber.org/zap/zapcore"
)

type LogLevel string

const (
	InfoLevel  LogLevel = "info"
	DebugLevel LogLevel = "debug"
)

func (l LogLevel) String() string {
	return map[LogLevel]string{
		InfoLevel:  "info",
		DebugLevel: "debug",
	}[l]
}

func (l LogLevel) ToZapLevel() zapcore.Level {
	switch l {
	case InfoLevel:
		return zapcore.InfoLevel
	case DebugLevel:
		return zapcore.DebugLevel
	default:
		return zapcore.InfoLevel
	}
}

type ReleaseLevel string

const (
	DevelopmentLevel ReleaseLevel = "development"
	ProductionLevel  ReleaseLevel = "production"
)

func (l ReleaseLevel) String() string {
	return map[ReleaseLevel]string{
		DevelopmentLevel: "development",
		ProductionLevel:  "production",
	}[l]
}
