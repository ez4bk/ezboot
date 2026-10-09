package main

import (
	"context"
	"log"

	"github.com/ez4bk/ezboot"
	"github.com/spf13/cobra"

	"github.com/ez4bk/ezboot/signal"

	"github.com/ez4bk/ezboot/cmd/ezboot/internal/project"

	"github.com/ez4bk/ezboot/cmd/ezboot/internal/model"
)

func main() {
	root := &cobra.Command{
		Use:           "ezboot",
		Short:         "An elegant toolkit for Golang microservice",
		Version:       ezboot.Version,
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
