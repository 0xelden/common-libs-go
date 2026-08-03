package worker

import (
	"time"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/hibiken/asynq"
)

type cronOption struct {
	uniqueTTL time.Duration
}

// CronOption mutates the private cronOption.
type CronOption func(opt *cronOption)

// WithUniqueTTL overrides the default dedup window (55s, just under cron's
// 1-minute resolution) used to guard a cron entry's enqueue when the
// scheduler itself runs on more than one replica. See RegisterCron.
func WithUniqueTTL(ttl time.Duration) CronOption {
	return func(o *cronOption) {
		o.uniqueTTL = ttl
	}
}

// RegisterCron arranges for a task to be enqueued on a recurring schedule
// (standard 5-field cron expression, e.g. "0 2 * * *", evaluated against
// APP_TIMEZONE — default Asia/Jakarta). `name` doubles as the task type and
// its dedicated queue, exactly like RegisterTask: a consumer must still call
// RegisterTask(name, handler, ...) to actually process the task —
// RegisterCron only arranges the enqueue, the same split as a crontab entry
// (schedule) vs. the script it invokes (handler).
//
// Safe to call on every replica of a horizontally scaled service: each tick
// is enqueued with asynq.Unique(ttl), keyed by queue+type+payload, so Redis
// accepts only the first of any replicas racing the same tick and rejects
// the rest as a duplicate (logged by asynq, not fatal. ttl defaults to 55s so it never bleeds into
// suppressing the *next* legitimate tick; override via WithUniqueTTL if your
// schedule ticks faster than once a minute.
func (cfg *Config) RegisterCron(spec, name string, payload []byte, option ...CronOption) serror.SError {
	opt := &cronOption{uniqueTTL: 55 * time.Second}
	for _, o := range option {
		o(opt)
	}

	if cfg.scheduler == nil {
		loc, _ := time.LoadLocation(helper.Env("APP_TIMEZONE", "Asia/Jakarta"))
		cfg.scheduler = asynq.NewScheduler(cfg.ClientOpt, &asynq.SchedulerOpts{Location: loc})
	}

	task := asynq.NewTask(name, payload)
	if _, err := cfg.scheduler.Register(spec, task, asynq.Queue(name), asynq.Unique(opt.uniqueTTL)); err != nil {
		return serror.Newf("register cron %q (%s): %v", name, spec, err)
	}
	return nil
}
