package worker

import (
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Worker struct {
	Scheduler *CronScheduler
	Store     *Store
}

type Config struct {
	CronPollEvery time.Duration
	CronLockID    int64
	ListenDSN     string
}

func DefaultConfig() Config {
	return Config{
		CronPollEvery: 10 * time.Minute,
		CronLockID:    2123216947,
	}
}

func New(pool *pgxpool.Pool, store *Store, handlers []HandlerDef, cfg Config, log *zap.Logger) *Worker {
	schedCfg := DefaultCronSchedulerConfig()
	schedCfg.FallbackInterval = cfg.CronPollEvery
	schedCfg.LockID = cfg.CronLockID
	schedCfg.ListenDSN = cfg.ListenDSN

	scheduler := NewCronScheduler(pool, store, handlers, schedCfg, log)
	store.SetWakeup(scheduler.Wakeup)

	return &Worker{
		Scheduler: scheduler,
		Store:     store,
	}
}

func (w *Worker) Start() {
	w.Scheduler.Start()
}

func (w *Worker) Stop() {
	w.Scheduler.Stop()
}
