package environment

import (
	"os"

	"video-optimization/internal/logger"
)

type Environment struct {
	ServerUrl               string
	R2Endpoint              string
	R2Bucket                string
	R2AccessKeyId           string
	R2SecretAccessKey       string
	VideoOptimizationApiKey string
}

var env *Environment

func Init() {
	secrets := map[string]*string{
		"R2_ENDPOINT":                nil,
		"R2_BUCKET":                  nil,
		"R2_ACCESS_KEY_ID":           nil,
		"R2_SECRET_ACCESS_KEY":       nil,
		"SERVER_URL":                 nil,
		"VIDEO_OPTIMIZATION_API_KEY": nil,
	}

	for key := range secrets {
		value := os.Getenv(key)
		if value == "" {
			logger.Logger.Fatalf("%s is not set", key)
		}
		secrets[key] = &value
	}

	env = &Environment{
		ServerUrl:               *secrets["SERVER_URL"],
		R2Endpoint:              *secrets["R2_ENDPOINT"],
		R2Bucket:                *secrets["R2_BUCKET"],
		R2AccessKeyId:           *secrets["R2_ACCESS_KEY_ID"],
		R2SecretAccessKey:       *secrets["R2_SECRET_ACCESS_KEY"],
		VideoOptimizationApiKey: *secrets["VIDEO_OPTIMIZATION_API_KEY"],
	}

	logger.Logger.Infow("Environment initialized",
		"ServerUrl", env.ServerUrl,
		"R2Endpoint", env.R2Endpoint,
		"R2Bucket", env.R2Bucket,
	)
}

func GetEnvironment() *Environment {
	return env
}
