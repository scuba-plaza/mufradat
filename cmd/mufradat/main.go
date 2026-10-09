package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/scuba-plaza/mufradat/internal/cli"
	"github.com/scuba-plaza/mufradat/internal/env"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Execute(ctx, env.OS(), os.Args[1:])
	stop()
	os.Exit(code)
}
