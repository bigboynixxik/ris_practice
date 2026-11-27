// Package app wires the dependencies and manages the application lifecycle.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ris/internal/models"
	"ris/internal/repository/postgres"
	"ris/internal/service"
	"ris/internal/transport/nobel"
	"ris/pkg/closer"
	"ris/pkg/config"
	"ris/pkg/logger"
)

// ErrInvalidArguments indicates invalid or conflicting command-line options.
var ErrInvalidArguments = errors.New("invalid command-line arguments")

// App loads a laureate dataset and saves it to PostgreSQL.
type App struct {
	cfg    *config.Config
	help   bool
	once   bool
	client *http.Client
	closer *closer.Closer
}

// New parses command-line options and initializes the application.
func New(args []string) (*App, error) {
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		return nil, fmt.Errorf("app.New load config: %w", err)
	}

	a := &App{
		cfg: cfg,
		client: &http.Client{
			Timeout:   cfg.HTTPTimeout,
			Transport: http.DefaultTransport.(*http.Transport).Clone(),
		},
		closer: closer.New(),
	}

	flags := flag.NewFlagSet("ris", flag.ContinueOnError)
	flags.BoolVar(&a.once, "once", false, "load one dataset and exit")
	flags.StringVar(&cfg.SourceURL, "url", cfg.SourceURL, "Nobel Prize API v1 JSON URL")
	flags.StringVar(&cfg.JSONFile, "file", cfg.JSONFile, "local Nobel Prize v1 JSON file")
	flags.StringVar(&cfg.DatabaseURL, "dsn", cfg.DatabaseURL, "PostgreSQL DSN")
	flags.StringVar(&cfg.LogPath, "log", cfg.LogPath, "append-only log file")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			a.help = true
			return a, nil
		}
		return nil, fmt.Errorf("app.New parse flags: %w", err)
	}
	if flags.NArg() > 0 {
		return nil, fmt.Errorf("app.New unexpected positional argument %q: %w", flags.Arg(0), ErrInvalidArguments)
	}
	var urlSet, fileSet bool
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "url":
			urlSet = true
		case "file":
			fileSet = true
		}
	})
	if urlSet && fileSet {
		return nil, fmt.Errorf("app.New choose either -url or -file: %w", ErrInvalidArguments)
	}
	if urlSet {
		cfg.JSONFile = ""
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("app.New validate config: %w", err)
	}
	return a, nil
}

// Run waits for shutdown or executes a single load requested by a demo command.
func (a *App) Run(ctx context.Context) (runErr error) {
	if a.help {
		return nil
	}

	ctx, stopSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	if err := os.MkdirAll(filepath.Dir(a.cfg.LogPath), 0o750); err != nil {
		return fmt.Errorf("app.Run create log directory: %w", err)
	}
	file, err := os.OpenFile(a.cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("app.Run open log file: %w", err)
	}
	defer func() {
		a.closer.Add(func(context.Context) error { return file.Close() })
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
		defer cancel()
		if closeErr := a.closer.Close(shutdownCtx); closeErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("app.Run shutdown: %w", closeErr))
		}
	}()

	a.closer.Add(func(context.Context) error {
		a.client.CloseIdleConnections()
		return nil
	})

	log := logger.NewForEnv(a.cfg.AppEnv, io.MultiWriter(os.Stdout, file), "ris")
	ctx = logger.WithContext(ctx, log)
	pool, err := openDatabase(ctx, a.cfg.DatabaseURL)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		log.ErrorContext(ctx, "database initialization failed", "error", err)
		return fmt.Errorf("app.Run open database: %w", err)
	}
	a.closer.Add(func(context.Context) error {
		pool.Close()
		return nil
	})
	if !a.once {
		log.InfoContext(ctx, "application ready")
		<-ctx.Done()
		log.InfoContext(ctx, "application stopped")
		return nil
	}
	log.InfoContext(ctx, "dataset loading started", "url", a.cfg.SourceURL, "file", a.cfg.JSONFile)

	laureates := service.NewLaureates(postgres.NewLaureatesRepo(pool), postgres.NewTxManager(pool))
	count, err := a.loadDataset(ctx, laureates)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			log.InfoContext(ctx, "dataset loading canceled")
			return nil
		}
		log.ErrorContext(ctx, "dataset loading failed", "error", err)
		return fmt.Errorf("app.Run load dataset: %w", err)
	}
	log.InfoContext(ctx, "dataset loading finished", "laureates", count)
	return nil
}

func (a *App) loadDataset(ctx context.Context, laureates *service.Laureates) (int, error) {
	var records []models.Laureate
	var err error
	if a.cfg.JSONFile != "" {
		records, err = nobel.ReadFile(ctx, a.cfg.JSONFile)
	} else {
		records, err = nobel.Download(ctx, a.client, a.cfg.SourceURL)
	}
	if err != nil {
		return 0, fmt.Errorf("app.loadDataset read laureates: %w", err)
	}
	logger.FromContext(ctx).InfoContext(ctx, "dataset decoded", "laureates", len(records))

	if err := laureates.SaveDataset(ctx, records); err != nil {
		return 0, fmt.Errorf("app.loadDataset save laureates: %w", err)
	}
	return len(records), nil
}
