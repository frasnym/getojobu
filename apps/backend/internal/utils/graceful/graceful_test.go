package graceful

import (
	"context"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// resetShutdownState keeps the process-wide readiness flag from leaking between
// tests, since StopProcess sets it by design.
func resetShutdownState(t *testing.T) {
	t.Helper()
	shuttingDown.Store(false)
	t.Cleanup(func() { shuttingDown.Store(false) })
}

func TestStopProcess_RunsStoppersInRegistrationOrder(t *testing.T) {
	resetShutdownState(t)

	var (
		mu    sync.Mutex
		order []string
	)
	record := func(name string) func(context.Context) error {
		return func(context.Context) error {
			mu.Lock()
			defer mu.Unlock()
			order = append(order, name)
			return nil
		}
	}

	StopProcess(context.Background(), 0, time.Second,
		record("server"), record("consumer"), record("contract"))

	// Registration order is shutdown order in this repo: the contract holds the
	// DB and Redis pools and must close after everything still using them.
	assert.Equal(t, []string{"server", "consumer", "contract"}, order)
}

func TestStopProcess_ContinuesPastAFailingStopper(t *testing.T) {
	resetShutdownState(t)

	contractClosed := false
	StopProcess(context.Background(), 0, time.Second,
		func(context.Context) error { return errors.New("server refused to close") },
		nil,
		func(context.Context) error { contractClosed = true; return nil },
	)

	assert.True(t, contractClosed, "a failing stopper must not strand the pools behind it")
}

func TestStopProcess_StopsOnceTheSharedBudgetIsExhausted(t *testing.T) {
	resetShutdownState(t)

	secondRan := false
	start := time.Now()

	StopProcess(context.Background(), 0, 50*time.Millisecond,
		func(ctx context.Context) error {
			<-ctx.Done() // outlive the whole budget, as a wedged drain would
			return ctx.Err()
		},
		func(context.Context) error { secondRan = true; return nil },
	)

	assert.False(t, secondRan, "budget is shared across stoppers, not granted to each")
	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

func TestStopProcess_FlipsReadinessBeforeDraining(t *testing.T) {
	resetShutdownState(t)
	assert.False(t, IsShuttingDown())

	drainingWhenStopperRan := false
	StopProcess(context.Background(), 20*time.Millisecond, time.Second,
		func(context.Context) error {
			drainingWhenStopperRan = IsShuttingDown()
			return nil
		})

	assert.True(t, drainingWhenStopperRan)
	assert.True(t, IsShuttingDown())
}

func TestStopProcess_WaitsOutTheDrainDelay(t *testing.T) {
	resetShutdownState(t)

	start := time.Now()
	StopProcess(context.Background(), 60*time.Millisecond, time.Second,
		func(context.Context) error { return nil })

	assert.GreaterOrEqual(t, time.Since(start), 60*time.Millisecond)
}

func TestStopProcess_DefaultsTheBudgetWhenUnset(t *testing.T) {
	resetShutdownState(t)

	ran := false
	StopProcess(context.Background(), 0, 0, func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		assert.True(t, ok, "an absent config value must not mean an unbounded shutdown")
		assert.WithinDuration(t, time.Now().Add(defaultStopTimeout), deadline, time.Second)
		ran = true
		return nil
	})

	assert.True(t, ran)
}

func TestStartProcessAtBackground_ReportsStarterFailures(t *testing.T) {
	resetShutdownState(t)

	want := errors.New("listen tcp :3000: bind: address already in use")
	errCh := StartProcessAtBackground(
		nil,
		func() error { return want },
		func() error { select {} }, // a healthy server never returns
	)

	select {
	case got := <-errCh:
		assert.Equal(t, want, got)
	case <-time.After(time.Second):
		t.Fatal("a failed starter must surface instead of hanging the process")
	}
}

func TestWaitForShutdownSignal_ReturnsOnStarterFailure(t *testing.T) {
	resetShutdownState(t)

	errCh := make(chan error, 1)
	errCh <- errors.New("bind failed")

	got := make(chan error, 1)
	go func() { got <- WaitForShutdownSignal(context.Background(), errCh) }()

	select {
	case err := <-got:
		// Reported, not swallowed: the caller exits non-zero on this, so a pod
		// that could not bind shows up as Error rather than Completed.
		assert.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("WaitForShutdownSignal ignored a starter failure")
	}
}

func TestWaitForShutdownSignal_ReturnsOnSigterm(t *testing.T) {
	resetShutdownState(t)

	got := make(chan error, 1)
	go func() { got <- WaitForShutdownSignal(context.Background(), nil) }()

	// Give the goroutine time to install its signal handler; without it the
	// SIGTERM lands on the default disposition and kills the test binary.
	time.Sleep(100 * time.Millisecond)
	assert.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case err := <-got:
		assert.NoError(t, err, "an ordinary termination signal is not a failure")
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdownSignal ignored SIGTERM")
	}
}
