package scdt

import (
	l "github.com/goextension/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log l.Logger

// logLevel ...
var logLevel = zapcore.DebugLevel

func init() {
	cfg := zap.NewProductionConfig()
	level := zap.NewAtomicLevel()
	level.SetLevel(logLevel)
	cfg.Level = level
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}
	logger, e := cfg.Build(
		zap.AddCaller(),
		//zap.AddCallerSkip(1),
	)
	if e != nil {
		panic(e)
	}
	log = logger.Sugar()
}
