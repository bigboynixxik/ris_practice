// Package cli handles dataset loading commands and logs their results.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"ris/internal/models"
	"ris/internal/transport/nobel"
	"ris/pkg/logger"
)

// DatasetService saves a decoded dataset in one transaction.
type DatasetService interface {
	SaveDataset(context.Context, []models.Laureate) error
}

// Handler connects a command's JSON source to the dataset service.
type Handler struct {
	client  *http.Client
	service DatasetService
	url     string
	file    string
}

// NewHandler constructs the command handler for an HTTP or local JSON source.
func NewHandler(client *http.Client, service DatasetService, url, file string) *Handler {
	return &Handler{client: client, service: service, url: url, file: file}
}

// Load reads and saves a dataset, logging any error once at the transport boundary.
func (h *Handler) Load(ctx context.Context) (count int, runErr error) {
	log := logger.FromContext(ctx).With("transport", "cli")
	ctx = logger.WithContext(ctx, log)
	log.InfoContext(ctx, "dataset loading started", "url", h.url, "file", h.file)
	defer func() {
		switch {
		case runErr == nil:
			log.InfoContext(ctx, "dataset loading finished", "laureates", count)
		case errors.Is(runErr, context.Canceled) && ctx.Err() != nil:
			log.InfoContext(ctx, "dataset loading canceled")
		default:
			log.ErrorContext(ctx, "dataset loading failed", "error", runErr)
		}
	}()

	var records []models.Laureate
	var err error
	if h.file != "" {
		records, err = nobel.ReadFile(ctx, h.file)
	} else {
		records, err = nobel.Download(ctx, h.client, h.url)
	}
	if err != nil {
		return 0, fmt.Errorf("cli.Handler.Load read dataset: %w", err)
	}
	log.InfoContext(ctx, "dataset decoded", "laureates", len(records))
	if err := h.service.SaveDataset(ctx, records); err != nil {
		return 0, fmt.Errorf("cli.Handler.Load save dataset: %w", err)
	}
	return len(records), nil
}
