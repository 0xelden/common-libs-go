package helper

import (
	_ "github.com/joho/godotenv/autoload"
	"os"
)

type environment uint8

const (
	local environment = iota
	dev
	qa
	prod
)

func getAppEnv() environment {
	switch NormalizeLower(os.Getenv("APP_ENV"), "") {
	case "prod", "production":
		return prod
	case "qa", "staging":
		return qa
	case "dev", "devel", "development":
		return dev
	default:
		return local
	}
}

func IsProd() bool  { return getAppEnv() == prod }
func IsQA() bool    { return getAppEnv() == qa }
func IsDev() bool   { return getAppEnv() == dev }
func IsLocal() bool { return getAppEnv() == local }
