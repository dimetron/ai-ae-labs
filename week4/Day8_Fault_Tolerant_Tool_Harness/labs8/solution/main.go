// Reference solution of Homework 8: a durable, fault-tolerant refund agent.
//
//	temporal server start-dev --db-filename .data/temporal.db   # durable execution (SQLite)
//	docker compose up -d dgraph                                  # long-term memory
//	go run ./solution worker                                     # terminal 1
//	go run ./solution start "Open a refund for txn-2026-07-118845 at merchant A-114"
//
// Wiring only; the logic lives in harness.go, tools.go, workflow.go,
// activities.go and app.go.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
