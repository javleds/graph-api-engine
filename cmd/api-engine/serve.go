package main

import (
	"context"
	"flag"
	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/platform/config"
	"os"
	"os/signal"
	"syscall"
)

func serve(args []string, migrateOnly bool) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	path := flags.String("config", "", "operator JSON configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if migrateOnly {
		return bootstrap.Migrate(ctx, cfg)
	}
	app, err := bootstrap.Build(ctx, cfg)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.Run(ctx)
}
