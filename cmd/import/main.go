// Command import downloads the laureate dataset and persists it in PostgreSQL.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"ris/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := app.RunImport(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
