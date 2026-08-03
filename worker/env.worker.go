package worker

import (
	"github.com/0xelden/common-libs-go/helper"
	_ "github.com/joho/godotenv/autoload"
)

const (
	EnvKeyJobStrictPriority = "JOB_STRICT_PRIORITY"
	EnvKeyJobWorkerSize     = "JOB_WORKER_SIZE"
)

var (
	// EnvJobWorkerSize define worker WorkerSize, default to 10 parallel worker
	EnvJobWorkerSize int = int(helper.StringToInt(helper.Env(EnvKeyJobWorkerSize), 10))

	// EnvJobStrictPriority define Queue processing on strict mode, default to true
	EnvJobStrictPriority bool = helper.Env(EnvKeyJobStrictPriority, "true") == "true"
)
