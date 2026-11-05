package graceful

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// defaultStopTimeout bounds the teardown when the caller passes no budget of its
// own, so a missing config value can never turn into an unbounded shutdown.
const defaultStopTimeout = 20 * time.Second

type ProcessStarter func() error

type ProcessStopper func(ctx context.Context) error

// shuttingDown is flipped once a termination signal has been observed. Readiness
// handlers read it so the pod reports 503 while it drains, which is what tells
// the load balancer and the service mesh to stop routing before the listener
// closes. It is process-wide because draining is a property of the process, not
// of any one handler: that keeps every entrypoint correct without threading a
// handler reference through api.New and InitRoute twice.
var shuttingDown atomic.Bool

// MarkShuttingDown puts the process into draining state. StopProcess calls it, so
// entrypoints get the readiness flip without wiring anything.
func MarkShuttingDown() {
	shuttingDown.Store(true)
}

// IsShuttingDown reports whether a termination signal has been observed.
func IsShuttingDown() bool {
	return shuttingDown.Load()
}

// StartProcessAtBackground runs every starter on its own goroutine and returns a
// channel carrying starter errors.
//
// Reading that channel is what stops a failed listener from hanging the process:
// previously a bind failure was discarded, so the pod stayed alive, never became
// ready, and was only killed once liveness gave up.
//
// Callers that have nothing to react with may ignore the channel; it is buffered
// to the number of starters so no goroutine leaks on an unread send. During a
// normal shutdown the starters also report here (echo returns
// http.ErrServerClosed, the Kafka subscriber returns a context cancellation) —
// that is expected, and by then nobody is listening.
func StartProcessAtBackground(ps ...func() error) <-chan error {
	errCh := make(chan error, len(ps))

	for _, p := range ps {
		if p == nil {
			continue
		}

		go func(_p func() error) {
			if err := _p(); err != nil {
				errCh <- err
			}
		}(p)
	}

	return errCh
}

// WaitForShutdownSignal blocks until SIGINT/SIGTERM arrives, or until a starter
// fails, whichever happens first. A nil starterErr means signals only.
//
// It returns nil for an ordinary signal and the starter's error otherwise, so
// the caller can exit non-zero. A process that dies because it could not bind
// its port must not report success: Kubernetes would show the pod as Completed
// rather than Error, and the reason would be buried in the logs.
func WaitForShutdownSignal(ctx context.Context, starterErr <-chan error) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(ch)

	select {
	case sig := <-ch:
		slog.WarnContext(ctx, "received signal, starting graceful shutdown", "signal", sig)
		return nil
	case err := <-starterErr:
		slog.ErrorContext(ctx, "starter failed, starting graceful shutdown", "error", err)
		return err
	}
}

// StopProcess flips readiness, waits out drainDelay, then runs the stoppers in
// registration order under a single shared timeout.
//
// drainDelay keeps the listener open and answering while Kubernetes finishes
// removing the pod from its Endpoints. Closing it the instant SIGTERM lands is
// what produces 502s and refused connections on every rollout and scale-down,
// because endpoint removal propagates asynchronously and in parallel with the
// signal. The preStop hook in the Helm values covers the same race one layer up;
// this covers callers that health-check us directly.
//
// Stoppers run in the order they were registered, which in this repo is already
// the order they must stop in: servers and consumers first, stopperContract last
// so the pools stay open until nothing is still using them. See
// codewiki/entities/entrypoint-bootstrap.md. Do not reverse this. Sibling
// services reverse because they register bottom-up; ours registers top-down, and
// flipping it would close the DB out from under a draining consumer.
//
// timeout is one budget for the whole sequence rather than per stopper. A
// per-stopper budget multiplies by the number of stoppers, and the product
// quietly exceeds the pod's termination grace period, which converts a graceful
// shutdown into a SIGKILL partway through.
func StopProcess(ctx context.Context, drainDelay, timeout time.Duration, ps ...ProcessStopper) {
	MarkShuttingDown()

	if drainDelay > 0 {
		slog.InfoContext(ctx, "draining before stopping services", "duration", drainDelay)
		time.Sleep(drainDelay)
	}

	if timeout <= 0 {
		timeout = defaultStopTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	total := len(ps)
	for i, p := range ps {
		if p == nil {
			continue
		}

		if err := ctx.Err(); err != nil {
			slog.ErrorContext(ctx, "shutdown budget exhausted", "stoppers_left", total-i, "error", err)
			return
		}

		// Carry on past a failure: the stoppers behind this one are usually the
		// connection pools, and stranding them leaks the very resources shutdown
		// exists to release.
		if err := p(ctx); err != nil {
			slog.ErrorContext(ctx, "stopper failed", "stopper", i+1, "total", total, "error", err)
			continue
		}

		slog.InfoContext(ctx, "stopper stopped", "stopper", i+1, "total", total)
	}
}

// StopProcessAtBackground waits for a termination signal and then stops every
// process. It is the single-call form, for entrypoints with no inbound traffic
// to drain.
func StopProcessAtBackground(timeout time.Duration, ps ...ProcessStopper) {
	ctx := context.Background()

	_ = WaitForShutdownSignal(ctx, nil)
	StopProcess(ctx, 0, timeout, ps...)
}
