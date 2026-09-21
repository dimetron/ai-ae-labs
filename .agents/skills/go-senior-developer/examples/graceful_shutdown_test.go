package examples

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestApplication_GracefulShutdown(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Use port 0 to let OS assign an available random port
	app := NewApplication("127.0.0.1:0", handler)

	ctx, cancel := context.WithCancel(context.Background())

	// Start background worker
	app.StartWorker(ctx, "metrics-collector", 10*time.Millisecond)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run(ctx, 2*time.Second)
	}()

	// Allow server and worker a moment to initialize
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() returned unexpected error on graceful shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not shut down within deadline")
	}
}
