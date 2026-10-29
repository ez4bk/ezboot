package main

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	"github.com/ez4bk/ezboot/signal"

	"github.com/ez4bk/ezboot/cmd/ezboot/internal/project"

	"github.com/ez4bk/ezboot/cmd/ezboot/internal/model"
)

func main() {
	const version = "v0.5.2"

	root := &cobra.Command{
		Use:           "ezboot",
		Short:         "An elegant toolkit for Golang microservice",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(project.CommandNew())
	root.AddCommand(model.Command())

	ctx, cancel := signal.WithExit(context.Background())
	defer cancel()

	if err := root.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
