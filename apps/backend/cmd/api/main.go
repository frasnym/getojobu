package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Assemble the contract (pure wiring — no DB connection yet).
	// stopperContract closes the DB pool and runs LAST so pools stay open
	// until nothing is still using them.
	c, stopperContract, err := contract.New(cfg)
	if err != nil {
		return err
	}

	starters := []func() error{}
	stoppers := []graceful.ProcessStopper{}

	// 3. Build the HTTP delivery (initializes the Neon DB pool into the contract).
	a, err := api.New(c)
	if err != nil {
		return err
	}

	// starter runs ListenAndServe; stopper shuts down server then closes DB pool.
	starterApi, stopperApi := a.StartApi()
	starters = append(starters, starterApi)
	stoppers = append(stoppers, stopperApi)

	// Shutdown contract dependency last to prevent panic (pools stay open
	// until nothing is still using them).
	stoppers = append(stoppers, graceful.ProcessStopper(stopperContract))

	// 4. Start serving: run starters in background, wait for SIGINT/SIGTERM,
	// then run stoppers under a shared shutdown timeout.
	starterErr := graceful.StartProcessAtBackground(starters...)
	startupFailure := graceful.WaitForShutdownSignal(ctx, starterErr)

	drainDelay := 500 * time.Millisecond
	if startupFailure != nil {
		drainDelay = 0
	}

	graceful.StopProcess(ctx, drainDelay, cfg.Server.ShutdownTimeout, stoppers...)
	cancel()

	if startupFailure != nil {
		return startupFailure
	}
	return nil
}
