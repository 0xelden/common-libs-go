package worker

import (
	"context"
	"log"

	"github.com/0xelden/common-libs-go/cli"
	"github.com/0xelden/common-libs-go/db"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	Name        string
	ClientOpt   *asynq.RedisClientOpt
	AsynqClient *asynq.Client
	Priority    map[string]int
	Parallel    map[string]int
	Mux         map[string]*asynq.ServeMux
	Trxb        db.TrxBuilder

	// scheduler is lazily created by the first RegisterCron call; nil means
	// no cron entries were registered and StartWorker won't start one.
	scheduler *asynq.Scheduler
}

func NewConfig(name string, clientOpt *asynq.RedisClientOpt, client *asynq.Client, trxb db.TrxBuilder) Config {
	cfg := Config{
		Name:        name,
		Priority:    map[string]int{},
		Parallel:    map[string]int{},
		Mux:         map[string]*asynq.ServeMux{},
		ClientOpt:   clientOpt,
		AsynqClient: client,
		Trxb:        trxb,
	}
	return cfg
}

func (cfg *Config) startWorker(ctx context.Context, name string, mux *asynq.ServeMux) serror.SError {
	wsize := helper.Coalesce(cfg.Parallel[name], 10)
	queue := map[string]int{name: helper.Coalesce(cfg.Priority[name], 10)}
	worker := asynq.NewServer(cfg.ClientOpt, asynq.Config{
		Concurrency:    wsize,
		Queues:         queue,
		StrictPriority: EnvJobStrictPriority,
	})

	log.Println("")
	log.Println("--------- " + name + " worker ---------")
	log.Printf("Running %d worker\n", wsize)
	log.Println("queue priority:", queue)

	// register task middleware
	// order is ipmortant, this one need to be registered after all handler available on the mux
	mux.Use(cfg.TracingMiddleware)
	if err := worker.Start(mux); err != nil {
		return serror.NewFromError(err)
	}

	<-ctx.Done()
	worker.Shutdown()
	return nil
}

// StartWorker runs every registered queue and the cron scheduler until ctx is
// cancelled, then shuts them down, letting in-flight tasks finish within
// asynq's ShutdownTimeout. It returns early if a queue or the scheduler fails
// to start.
func (cfg *Config) StartWorker(ctx context.Context) serror.SError {
	log.Println("Workers version:", cli.Version, "commit id:", cli.Commit, "built:", cli.Build)
	log.Printf(`Strict mode: %v`, EnvJobStrictPriority)
	g, ctx := errgroup.WithContext(ctx)
	for k, v := range cfg.Mux {
		k := k
		v := v
		g.Go(func() error {
			return cfg.startWorker(ctx, k, v)
		})
	}

	if cfg.scheduler != nil {
		g.Go(func() error {
			if err := cfg.scheduler.Start(); err != nil {
				return err
			}
			<-ctx.Done()
			cfg.scheduler.Shutdown()
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return serror.NewFromError(err)
	}
	return nil
}
