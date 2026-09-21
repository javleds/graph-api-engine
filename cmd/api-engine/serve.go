package main

import (
	"context"
	"github.com/javleds/api-engine/internal/bootstrap"
	"github.com/javleds/api-engine/internal/platform/config"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newServeCommand(name string, migrateOnly bool) *cobra.Command {
	var path string
	command := &cobra.Command{
		Use:   name,
		Short: commandSummary(name),
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return runServe(path, migrateOnly)
		},
	}
	command.Flags().StringVar(&path, "config", "", "operator YAML configuration")
	return command
}

func commandSummary(name string) string {
	if name == "migrate" {
		return "Apply storage migrations"
	}
	return "Run the API server"
}

func runServe(path string, migrateOnly bool) error {
	cfg, err := config.Load(path)
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
