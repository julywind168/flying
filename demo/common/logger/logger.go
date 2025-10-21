package logger

import "go.uber.org/zap"

var Sugar *zap.SugaredLogger

func init() {
	logger, _ := zap.NewDevelopment()
	Sugar = logger.Sugar()
}

func Sync() error {
	return Sugar.Sync()
}
