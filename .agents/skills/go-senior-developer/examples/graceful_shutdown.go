package examples

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Application encapsulates the HTTP server and background worker lifecycle.
type Application struct {
	server   *http.Server
	workerWg sync.WaitGroup
}

// NewApplication constructs a production-ready application runner.
func NewApplication(addr string, handler http.Handler) *Application {
	return &Application{
		server: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 3 * time.Second,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
		},
	}
}

// StartWorker launches a background worker that terminates when ctx is cancelled.
func (app *Application) StartWorker(ctx context.Context, name string, workInterval time.Duration) {
	app.workerWg.Add(1)
	go func() {
		defer app.workerWg.Done()
		ticker := time.NewTicker(workInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Simulate periodic task
			}
		}
	}()
}

// Run executes the server and blocks until ctx is cancelled, then executes graceful shutdown.
func (app *Application) Run(ctx context.Context, shutdownTimeout time.Duration) error {
	serverErr := make(chan error, 1)

	// 1. Launch HTTP server in background
	go func() {
		if err := app.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	// 2. Block until context cancellation or server startup crash
	select {
	case err := <-serverErr:
		return fmt.Errorf("http server crashed: %w", err)
	case <-ctx.Done():
		// Context cancelled, initiate graceful shutdown
	}

	// 3. Drain active HTTP connections with bounded deadline
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var shutdownErr error
	if err := app.server.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("graceful shutdown failed: %w", err)
	}

	// 4. Wait for background workers to finish
	app.workerWg.Wait()

	return shutdownErr
}
