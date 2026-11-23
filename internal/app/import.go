package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"ris/internal/logger"
	"ris/internal/repository/postgres"
	"ris/internal/service"
	"ris/internal/transport/nobel"
)

// RunImport executes the import command.
func RunImport(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	sourceURL := flags.String("url", nobel.DefaultURL, "Nobel Prize API v1 JSON URL")
	dsn := flags.String("dsn", env("RIS_DATABASE_URL", defaultDSN), "PostgreSQL DSN")
	logPath := flags.String("log", "import.log", "append-only log file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("app.RunImport parse flags: %w", err)
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("app.RunImport unexpected positional arguments: %w", flag.ErrHelp)
	}

	file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("app.RunImport open log file: %w", err)
	}
	defer file.Close()
	ctx = logger.WithContext(ctx, slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, file), nil)))
	log := logger.FromContext(ctx)
	log.InfoContext(ctx, "import started", "url", *sourceURL)

	count, err := importDataset(ctx, *sourceURL, *dsn)
	if err != nil {
		log.ErrorContext(ctx, "import failed", "error", err)
		return fmt.Errorf("app.RunImport import dataset: %w", err)
	}
	log.InfoContext(ctx, "import finished", "laureates", count)
	return nil
}

func importDataset(ctx context.Context, sourceURL, dsn string) (int, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	records, err := nobel.Download(ctx, client, sourceURL)
	if err != nil {
		return 0, fmt.Errorf("app.importDataset download Nobel dataset: %w", err)
	}
	logger.FromContext(ctx).InfoContext(ctx, "dataset decoded", "laureates", len(records))

	pool, err := openDatabase(ctx, dsn)
	if err != nil {
		return 0, fmt.Errorf("app.importDataset open database: %w", err)
	}
	defer pool.Close()

	service := service.NewLaureates(postgres.New(pool))
	if err := service.Import(ctx, records); err != nil {
		return 0, fmt.Errorf("app.importDataset write dataset: %w", err)
	}
	return len(records), nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
