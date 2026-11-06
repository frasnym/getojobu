package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/graceful"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// API is the HTTP delivery. It owns the router, middleware chain, and
// http.Server. StartApi returns a starter and stopper for the graceful
// shutdown lifecycle orchestrated by graceful.StartProcessAtBackground and
// graceful.StopProcess in the entrypoint.
type API struct {
	cfg      *config.Config
	contract *contract.Contract
	rt       *httpx.Router
	server   *http.Server
}

// New assembles the HTTP delivery from the full contract.
// Contract wiring (contract.New) is separate from DB initialization;
// New initializes the Neon database pool via common.New into the contract's
// common layer. The DB stopper is owned by contract.New's stopper and runs
// last in the shutdown sequence, so pools stay open until nothing uses them.
func New(c *contract.Contract) (*API, error) {
	if c == nil {
		return nil, errors.New("contract is required")
	}
	cfg := c.Cfg
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	// Initialize the Neon database pool separately from contract wiring.
	// common.New returns the pool plus its own stopper, but the stopper is
	// intentionally not used here — contract.New's stopper closes the DB last
	// in the shutdown sequence (server stops first, then DB).
	dbContract, _, err := common.New(cfg)
	if err != nil {
		return nil, err
	}
	c.Common.DB = dbContract.DB

	// Set up structured logging.
	logger := logging.New(slog.LevelInfo)
	slog.SetDefault(logger)

	// Build router and register routes.
	rt := httpx.New()
	handler.New(c).Route(rt)

	// Assemble middleware chain: envelope (404/405 → JSON) → mux → logging → request ID → recovery.
	var handlerChain http.Handler = rt.EnvelopeHandler(rt.Mux())
	handlerChain = middleware.Logging(handlerChain)
	handlerChain = middleware.RequestID(handlerChain)
	handlerChain = middleware.Recovery(handlerChain)

	server := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handlerChain,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return &API{
		cfg:      cfg,
		contract: c,
		rt:       rt,
		server:   server,
	}, nil
}

// StartApi returns a starter and stopper for graceful shutdown lifecycle.
//
// starter runs ListenAndServe and reports errors on the starterErr channel.
// stopper shuts down the HTTP server under a shared timeout, orchestrated by
// graceful.StopProcess in the entrypoint. The database pool is closed by
// contract.New's stopper, which runs after the server stopper.
func (a *API) StartApi() (func() error, graceful.ProcessStopper) {
	starter := func() error {
		slog.Info("api starting", "addr", a.cfg.Server.Addr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped unexpectedly", "error", err)
			return err
		}
		return nil
	}

	stopper := graceful.ProcessStopper(func(ctx context.Context) error {
		// Drain briefly so the load balancer stops routing before we close
		// the listener.
		drainDelay := 500 * time.Millisecond
		if drainDelay > 0 {
			time.Sleep(drainDelay)
		}
		graceful.MarkShuttingDown()
		if err := a.server.Shutdown(ctx); err != nil {
			slog.Error("server shutdown failed", "error", err)
			return err
		}
		slog.Info("server stopped")
		return nil
	})

	return starter, stopper
}

// Stop triggers an immediate shutdown of the HTTP server.
func (a *API) Stop(ctx context.Context) error {
	if a == nil || a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// Close releases all closable resources (database pool).
func (a *API) Close() {
	if a == nil {
		return
	}
	_ = a.server.Shutdown(context.Background())
}

// Router returns the registered router for introspection or testing.
func (a *API) Router() *httpx.Router {
	return a.rt
}

// Contract returns the underlying contract for dependency access.
func (a *API) Contract() *contract.Contract {
	return a.contract
}
