package scdt

import (
	l "github.com/goextension/log"
	"go.uber.org/zap"
)

var log l.Logger

func init() {
	logger, e := zap.NewProduction(
		zap.AddCaller(),
	)
	if e != nil {
		panic(e)
	}
	l.Register(logger.Sugar())
	log = l.Log()
}
