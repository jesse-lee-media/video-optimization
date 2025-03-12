package logger

import (
	"os"

	"go.uber.org/zap"
)

var Logger *zap.SugaredLogger

func Init() {
	env := os.Getenv("APP_ENV")
	var log *zap.Logger
	var err error

	if env == "production" {
		log, err = zap.NewProduction()
	} else {
		log, err = zap.NewDevelopment()
	}

	if err != nil {
		panic(err)
	}

	Logger = log.Sugar()
	Logger.Infof("Logger initialized in %s mode", env)
}
