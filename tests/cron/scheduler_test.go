package cron_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tgdrive/teldrive/internal/database"
	"github.com/tgdrive/teldrive/pkg/worker"
	"github.com/tgdrive/teldrive/tests/testdb"
	"go.uber.org/zap"
)

func setupCronTest(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	pool := database.NewTestDatabase(t, true)
	testdb.Reset(t, pool)

	return pool, func() { pool.Close() }
}

func insertJob(t *testing.T, pool *pgxpool.Pool, kind string, cronExpr string, nextRunAt time.Time) string {
	t.Helper()
	id := uuid.NewString()
	if cronExpr == "" {
		cronExpr = "*/1 * * * *"
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO periodic_jobs (id, user_id, name, kind, args, cron_expression, enabled, next_run_at)
		VALUES ($1, 1, $2, $3, '{}'::jsonb, $4, true, $5)
	`, id, kind, kind, cronExpr, nextRunAt)
	require.NoError(t, err)
	return id
}

func TestStoreListDue(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	store := worker.NewStore(pool)

	future := time.Now().UTC().Add(1 * time.Hour)
	past := time.Now().UTC().Add(-1 * time.Minute)

	insertJob(t, pool, "test.due", "*/1 * * * *", past)
	insertJob(t, pool, "test.future", "*/1 * * * *", future)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO periodic_jobs (id, user_id, name, kind, args, cron_expression, enabled, next_run_at)
		VALUES (gen_random_uuid(), 1, 'test.disabled', 'test.disabled', '{}'::jsonb, '*/1 * * * *', false, $1)
	`, past)
	require.NoError(t, err)

	jobs, err := store.ListDue(context.Background(), 50)
	require.NoError(t, err)

	var kinds []string
	for _, j := range jobs {
		kinds = append(kinds, j.Kind)
	}
	assert.Contains(t, kinds, "test.due")
	assert.NotContains(t, kinds, "test.future")
	assert.NotContains(t, kinds, "test.disabled")
}

func TestStoreWakeup(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	store := worker.NewStore(pool)

	called := make(chan struct{}, 1)
	store.SetWakeup(func() {
		called <- struct{}{}
	})

	store.Wakeup()

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected wakeup callback to be called")
	}
}

func TestStoreMarkDueNowTriggersWakeup(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	id := insertJob(t, pool, "test.wakeup", "*/5 * * * *", time.Now().UTC().Add(1*time.Hour))

	store := worker.NewStore(pool)

	called := make(chan struct{}, 1)
	store.SetWakeup(func() {
		called <- struct{}{}
	})

	err := store.MarkDueNow(context.Background(), uuid.MustParse(id), 1)
	require.NoError(t, err)

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("expected MarkDueNow to trigger wakeup")
	}

	var nextRunAt time.Time
	err = pool.QueryRow(context.Background(),
		"SELECT next_run_at FROM periodic_jobs WHERE id = $1", id).Scan(&nextRunAt)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), nextRunAt, 2*time.Second)
}

func TestSchedulerRunsDueJob(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	handlerCalled := make(chan struct{}, 1)
	handlers := []worker.HandlerDef{
		{Kind: "test.run", Handler: func(ctx context.Context, job *worker.Job) error {
			handlerCalled <- struct{}{}
			return nil
		}},
	}

	store := worker.NewStore(pool)
	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	scheduler := worker.NewCronScheduler(pool, store, handlers, cfg, zap.NewNop())
	store.SetWakeup(scheduler.Wakeup)

	scheduler.Start()
	defer scheduler.Stop()

	insertJob(t, pool, "test.run", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	scheduler.Wakeup()

	select {
	case <-handlerCalled:
	case <-time.After(3 * time.Second):
		t.Fatal("expected job handler to be called within timeout")
	}
}

func TestSchedulerDoesNotRunJobWhenDisabled(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	handlerCalled := make(chan struct{}, 1)
	handlers := []worker.HandlerDef{
		{Kind: "test.disabled", Handler: func(ctx context.Context, job *worker.Job) error {
			handlerCalled <- struct{}{}
			return nil
		}},
	}

	store := worker.NewStore(pool)
	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	scheduler := worker.NewCronScheduler(pool, store, handlers, cfg, zap.NewNop())
	store.SetWakeup(scheduler.Wakeup)

	scheduler.Start()
	defer scheduler.Stop()

	id := uuid.NewString()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO periodic_jobs (id, user_id, name, kind, args, cron_expression, enabled, next_run_at)
		VALUES ($1, 1, 'test.disabled', 'test.disabled', '{}'::jsonb, '*/1 * * * *', false, NOW())
	`, id)
	require.NoError(t, err)

	scheduler.Wakeup()

	select {
	case <-handlerCalled:
		t.Fatal("disabled job should not run")
	case <-time.After(time.Second):
	}
}

func TestSchedulerRecordsJobState(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	handlers := []worker.HandlerDef{
		{Kind: "test.state", Handler: func(ctx context.Context, job *worker.Job) error {
			return nil
		}},
	}

	store := worker.NewStore(pool)
	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	scheduler := worker.NewCronScheduler(pool, store, handlers, cfg, zap.NewNop())
	store.SetWakeup(scheduler.Wakeup)

	scheduler.Start()
	defer scheduler.Stop()

	id := insertJob(t, pool, "test.state", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	scheduler.Wakeup()

	require.Eventually(t, func() bool {
		var lastState string
		err := pool.QueryRow(context.Background(),
			"SELECT last_state FROM periodic_jobs WHERE id = $1", id).Scan(&lastState)
		if err != nil {
			return false
		}
		return lastState == "succeeded"
	}, 3*time.Second, 100*time.Millisecond, "expected job state to become 'succeeded'")
}

func TestSchedulerRecordsJobFailed(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	handlers := []worker.HandlerDef{
		{Kind: "test.fail", Handler: func(ctx context.Context, job *worker.Job) error {
			return assert.AnError
		}},
	}

	store := worker.NewStore(pool)
	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	scheduler := worker.NewCronScheduler(pool, store, handlers, cfg, zap.NewNop())
	store.SetWakeup(scheduler.Wakeup)

	scheduler.Start()
	defer scheduler.Stop()

	id := insertJob(t, pool, "test.fail", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	scheduler.Wakeup()

	require.Eventually(t, func() bool {
		var lastState string
		err := pool.QueryRow(context.Background(),
			"SELECT last_state FROM periodic_jobs WHERE id = $1", id).Scan(&lastState)
		if err != nil {
			return false
		}
		return lastState == "failed"
	}, 3*time.Second, 100*time.Millisecond, "expected job state to become 'failed'")
}

func TestSchedulerRecordsLastError(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	handlers := []worker.HandlerDef{
		{Kind: "test.errored", Handler: func(ctx context.Context, job *worker.Job) error {
			return assert.AnError
		}},
	}

	store := worker.NewStore(pool)
	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	scheduler := worker.NewCronScheduler(pool, store, handlers, cfg, zap.NewNop())
	store.SetWakeup(scheduler.Wakeup)

	scheduler.Start()
	defer scheduler.Stop()

	id := insertJob(t, pool, "test.errored", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	scheduler.Wakeup()

	require.Eventually(t, func() bool {
		var lastError *string
		err := pool.QueryRow(context.Background(),
			"SELECT last_error FROM periodic_jobs WHERE id = $1", id).Scan(&lastError)
		if err != nil || lastError == nil {
			return false
		}
		return len(*lastError) > 0
	}, 3*time.Second, 100*time.Millisecond, "expected job to record last_error")
}

func TestSchedulerAdvisoryLock(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	var callCount atomic.Int64
	handlers := []worker.HandlerDef{
		{Kind: "test.lock", Handler: func(ctx context.Context, job *worker.Job) error {
			callCount.Add(1)
			time.Sleep(100 * time.Millisecond)
			return nil
		}},
	}

	store1 := worker.NewStore(pool)
	store2 := worker.NewStore(pool)

	cfg := worker.DefaultCronSchedulerConfig()
	cfg.ListenDSN = ""

	sched1 := worker.NewCronScheduler(pool, store1, handlers, cfg, zap.NewNop())
	sched2 := worker.NewCronScheduler(pool, store2, handlers, cfg, zap.NewNop())

	store1.SetWakeup(sched1.Wakeup)

	sched1.Start()
	sched2.Start()
	defer sched1.Stop()
	defer sched2.Stop()

	insertJob(t, pool, "test.lock", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	sched1.Wakeup()
	sched2.Wakeup()

	time.Sleep(500 * time.Millisecond)

	calls := callCount.Load()
	assert.Equal(t, int64(1), calls, "expected exactly one scheduler to run the job due to advisory lock")
}

func TestListenNotifyTrigger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LISTEN/NOTIFY trigger test in short mode")
	}

	pool, cleanup := setupCronTest(t)
	defer cleanup()

	listenConn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer listenConn.Release()

	_, err = listenConn.Exec(context.Background(), "LISTEN periodic_job_changed")
	require.NoError(t, err)

	id := uuid.NewString()
	_, err = pool.Exec(context.Background(), `
		INSERT INTO periodic_jobs (id, user_id, name, kind, args, cron_expression, enabled, next_run_at)
		VALUES ($1, 1, 'test.notify', 'test.notify', '{}'::jsonb, '*/1 * * * *', true, NOW())
	`, id)
	require.NoError(t, err)

	notification, err := listenConn.Conn().WaitForNotification(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "periodic_job_changed", notification.Channel)
	assert.Equal(t, id, notification.Payload)
}

func TestStoreMarkRunning(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	store := worker.NewStore(pool)

	id := insertJob(t, pool, "test.runmark", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	now := time.Now().UTC()
	err := store.MarkRunning(context.Background(), id, now)
	require.NoError(t, err)

	var lastState string
	var lastRunAt time.Time
	err = pool.QueryRow(context.Background(),
		"SELECT last_state, last_run_at FROM periodic_jobs WHERE id = $1", id).Scan(&lastState, &lastRunAt)
	require.NoError(t, err)
	assert.Equal(t, "running", lastState)
	assert.WithinDuration(t, now, lastRunAt, time.Second)
}

func TestStoreMarkSucceeded(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	store := worker.NewStore(pool)

	id := insertJob(t, pool, "test.succmark", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	nextRun := time.Now().UTC().Add(1 * time.Hour)
	err := store.MarkSucceeded(context.Background(), id, nextRun)
	require.NoError(t, err)

	var lastState string
	var actualNextRun time.Time
	err = pool.QueryRow(context.Background(),
		"SELECT last_state, next_run_at FROM periodic_jobs WHERE id = $1", id).Scan(&lastState, &actualNextRun)
	require.NoError(t, err)
	assert.Equal(t, "succeeded", lastState)
	assert.WithinDuration(t, nextRun, actualNextRun, time.Second)
}

func TestStoreMarkFailed(t *testing.T) {
	pool, cleanup := setupCronTest(t)
	defer cleanup()

	store := worker.NewStore(pool)

	id := insertJob(t, pool, "test.failmark", "*/1 * * * *", time.Now().UTC().Add(-1*time.Minute))

	nextRun := time.Now().UTC().Add(1 * time.Hour)
	err := store.MarkFailed(context.Background(), id, nextRun, "something went wrong")
	require.NoError(t, err)

	var lastState string
	var lastError *string
	err = pool.QueryRow(context.Background(),
		"SELECT last_state, last_error FROM periodic_jobs WHERE id = $1", id).Scan(&lastState, &lastError)
	require.NoError(t, err)
	assert.Equal(t, "failed", lastState)
	require.NotNil(t, lastError)
	assert.Equal(t, "something went wrong", *lastError)
}
