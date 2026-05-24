package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type CronScheduler struct {
	pool     *pgxpool.Pool
	store    *Store
	handlers map[string]Handler
	cfg      CronSchedulerConfig
	log      *zap.Logger
	parser   cron.Parser

	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	wakeup chan struct{}
}

type CronSchedulerConfig struct {
	FallbackInterval time.Duration
	LockID           int64
	ListenDSN        string
}

func DefaultCronSchedulerConfig() CronSchedulerConfig {
	return CronSchedulerConfig{
		FallbackInterval: 10 * time.Minute,
		LockID:           2123216947,
	}
}

func NewCronScheduler(pool *pgxpool.Pool, store *Store, handlers []HandlerDef, cfg CronSchedulerConfig, log *zap.Logger) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	handlerMap := make(map[string]Handler, len(handlers))
	for _, h := range handlers {
		handlerMap[h.Kind] = h.Handler
	}
	return &CronScheduler{
		pool:     pool,
		store:    store,
		handlers: handlerMap,
		cfg:      cfg,
		log:      log.Named("worker.cron"),
		parser:   cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
		ctx:      ctx,
		cancel:   cancel,
		wakeup:   make(chan struct{}, 1),
	}
}

func (s *CronScheduler) Wakeup() {
	select {
	case s.wakeup <- struct{}{}:
	default:
	}
}

func (s *CronScheduler) Start() {
	if s.running.Swap(true) {
		return
	}
	s.log.Info("starting cron background jobs")

	s.startListen()

	s.wg.Add(1)
	go s.scheduleLoop()
}

func (s *CronScheduler) Stop() {
	if !s.running.Swap(false) {
		return
	}
	s.cancel()
	s.wg.Wait()
	s.log.Info("cron background jobs stopped")
}

func (s *CronScheduler) startListen() {
	if s.cfg.ListenDSN == "" {
		return
	}

	conn, err := pgx.Connect(s.ctx, s.cfg.ListenDSN)
	if err != nil {
		s.log.Warn("cannot create LISTEN connection, falling back to timer-only", zap.Error(err))
		return
	}

	if _, err := conn.Exec(s.ctx, "LISTEN periodic_job_changed"); err != nil {
		conn.Close(s.ctx)
		s.log.Warn("cannot LISTEN on periodic_job_changed, falling back to timer-only", zap.Error(err))
		return
	}

	s.log.Info("LISTEN on periodic_job_changed established")

	lctx, lcancel := context.WithCancel(s.ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer conn.Close(lctx)
		defer lcancel()

		for {
			_, err := conn.WaitForNotification(lctx)
			if err != nil {
				if lctx.Err() != nil {
					return
				}
				s.log.Warn("NOTIFY connection lost, falling back to timer-only", zap.Error(err))
				return
			}
			select {
			case s.wakeup <- struct{}{}:
			default:
			}
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-s.ctx.Done()
		lcancel()
	}()
}

func (s *CronScheduler) scheduleLoop() {
	defer s.wg.Done()

	fallbackTicker := time.NewTicker(s.cfg.FallbackInterval)
	defer fallbackTicker.Stop()

	s.tryRunDueJobs()

	nextTimer := time.NewTimer(s.nextJobDelay())
	defer nextTimer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-nextTimer.C:
			s.tryRunDueJobs()
			nextTimer.Reset(s.nextJobDelay())

		case <-fallbackTicker.C:
			s.log.Debug("fallback ticker fired, checking due jobs")
			s.tryRunDueJobs()
			nextTimer.Reset(s.nextJobDelay())

		case <-s.wakeup:
			s.log.Debug("wakeup signal received, checking due jobs")
			s.tryRunDueJobs()
			nextTimer.Reset(s.nextJobDelay())
		}
	}
}

func (s *CronScheduler) nextJobDelay() time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var nextRunAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT MIN(next_run_at)
		FROM periodic_jobs
		WHERE enabled = true
	`).Scan(&nextRunAt)
	if err != nil || nextRunAt == nil {
		return 30 * time.Second
	}

	now := time.Now()
	if nextRunAt.Before(now) {
		return time.Second
	}

	delay := nextRunAt.Sub(now)
	if delay > 1*time.Hour {
		delay = 1 * time.Hour
	}
	return delay
}

func (s *CronScheduler) tryRunDueJobs() {
	conn, err := s.pool.Acquire(s.ctx)
	if err != nil {
		s.log.Error("acquire connection for advisory lock", zap.Error(err))
		return
	}
	defer conn.Release()

	var acquired bool
	if err := conn.QueryRow(s.ctx, "SELECT pg_try_advisory_lock($1)", s.cfg.LockID).Scan(&acquired); err != nil || !acquired {
		return
	}
	defer func() {
		_, _ = conn.Exec(s.ctx, "SELECT pg_advisory_unlock($1)", s.cfg.LockID)
	}()

	jobs, err := s.store.ListDue(s.ctx, 50)
	if err != nil {
		s.log.Error("query due background jobs", zap.Error(err))
		return
	}
	for _, job := range jobs {
		s.run(job)
	}
}

func (s *CronScheduler) run(cronJob *CronJob) {
	handler := s.handlers[cronJob.Kind]
	if handler == nil {
		s.log.Warn("no handler for background job", zap.String("kind", cronJob.Kind), zap.String("id", cronJob.ID))
		nextRunAt := s.nextRun(cronJob)
		_ = s.store.MarkFailed(s.ctx, cronJob.ID, nextRunAt, fmt.Sprintf("no handler registered for kind: %s", cronJob.Kind))
		return
	}

	startedAt := time.Now().UTC()
	if err := s.store.MarkRunning(s.ctx, cronJob.ID, startedAt); err != nil {
		s.log.Error("mark background job running", zap.String("id", cronJob.ID), zap.Error(err))
		return
	}

	args := s.buildJobArgs(cronJob)
	if args == nil {
		nextRunAt := s.nextRun(cronJob)
		_ = s.store.MarkFailed(s.ctx, cronJob.ID, nextRunAt, "invalid background job args")
		return
	}

	job := &Job{ID: cronJob.ID, UserID: cronJob.UserID, Kind: cronJob.Kind, Args: args, State: JobStateRunning}
	if err := handler(s.ctx, job); err != nil {
		nextRunAt := s.nextRun(cronJob)
		if storeErr := s.store.MarkFailed(s.ctx, cronJob.ID, nextRunAt, err.Error()); storeErr != nil {
			s.log.Error("mark background job failed", zap.String("id", cronJob.ID), zap.Error(storeErr))
		}
		s.log.Error("background job failed", zap.String("id", cronJob.ID), zap.String("kind", cronJob.Kind), zap.Error(err))
		return
	}

	nextRunAt := s.nextRun(cronJob)
	if err := s.store.MarkSucceeded(s.ctx, cronJob.ID, nextRunAt); err != nil {
		s.log.Error("mark background job succeeded", zap.String("id", cronJob.ID), zap.Error(err))
	}
}

func (s *CronScheduler) nextRun(cronJob *CronJob) time.Time {
	schedule, err := s.parser.Parse(cronJob.CronExpression)
	if err != nil {
		return time.Now().UTC().Add(s.cfg.FallbackInterval)
	}
	return schedule.Next(time.Now().UTC())
}

func (s *CronScheduler) buildJobArgs(cronJob *CronJob) []byte {
	raw := make(map[string]any)
	if len(cronJob.Args) > 0 {
		if err := json.Unmarshal(cronJob.Args, &raw); err != nil {
			raw = make(map[string]any)
		}
	}
	raw["userId"] = cronJob.UserID
	b, _ := json.Marshal(raw)
	return b
}
