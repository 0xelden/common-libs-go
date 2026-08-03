package worker

import (
	"fmt"
	"log"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/hibiken/asynq"
)

type taskOption struct {
	prioritySize int64
	parallelSize int64
	disable      bool
}

// TaskOption mutates the private taskConfig.
type TaskOption func(opt *taskOption) error

// RegisterTask register and set priority for this task
type RegisterTask func(name string, handler asynq.HandlerFunc, option ...TaskOption)

// RegisterTask register and set priority for this task, default to initialPriority
// example: 3 initialPriority means it'll be processed 30% of the time
// override using task name as environment value, task name as key and integer as value
func (cfg *Config) RegisterTask(name string, handler asynq.HandlerFunc, option ...TaskOption) {
	var (
		disable   = helper.NormalizeLower(helper.Env(helper.ToEnvKey(name+"_disable")), "")
		isDisable = disable == "1" || disable == "y" || disable == "yes" || disable == "true"
	)

	if isDisable {
		log.Printf("worker handler env for %s is disabled\n", name)
		return
	}

	opt := &taskOption{
		prioritySize: 10,
		parallelSize: 10,
		disable:      false,
	}
	for _, v := range option {
		err := v(opt)
		if err != nil {
			log.Panicln(err)
		}
	}

	if opt.disable {
		log.Printf("worker handler opt for %s is disabled\n", name)
		return
	}

	cfg.Priority[name] = int(helper.StringToInt(helper.Env(helper.ToEnvKey(name+"_priority")), opt.prioritySize))
	cfg.Parallel[name] = int(helper.StringToInt(helper.Env(helper.ToEnvKey(name+"_parallel")), opt.parallelSize))
	if cfg.Mux[name] == nil {
		cfg.Mux[name] = asynq.NewServeMux()
	}
	cfg.Mux[name].Handle(name, handler)
}

// WithPriority overrides the default priority (lower number == higher priority).
// example: 3 initialPriority means it'll be processed 30% of the time
// override using task name as environment value, task name as key and integer as value
func WithPriority(p int) TaskOption {
	return func(t *taskOption) error {
		if p < 1 || p > 10 {
			return fmt.Errorf("priority %d out of range (1-10)", p)
		}
		t.prioritySize = int64(p)
		return nil
	}
}

// WithParallel overrides the default worker parallel size
func WithParallel(p int) TaskOption {
	return func(t *taskOption) error {
		if p < 1 || p > 50 {
			return fmt.Errorf("parallel %d out of range (1-50)", p)
		}
		t.parallelSize = int64(p)
		return nil
	}
}

func WithDisableOpt(disable bool) TaskOption {
	return func(t *taskOption) error {
		t.disable = disable
		return nil
	}
}
