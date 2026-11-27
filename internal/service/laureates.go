// Package service implements operations on the laureate dataset.
package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"ris/internal/models"
)

var countryPattern = regexp.MustCompile(`^[A-Z]{2}$`)
var idPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

// Repository supplies the dataset operations used by Laureates.
type Repository interface {
	Insert(context.Context, models.Laureate) (bool, error)
	Update(context.Context, models.Laureate) error
	ReplacePrizes(context.Context, models.Laureate) error
	SetLastUpdated(context.Context) error
}

// TxManager groups a service operation into one transaction.
type TxManager interface {
	WithTx(context.Context, func(context.Context) error) error
}

// Laureates validates inputs and coordinates dataset operations.
type Laureates struct {
	repository Repository
	txManager  TxManager
}

// NewLaureates constructs the dataset service.
func NewLaureates(repository Repository, txManager TxManager) *Laureates {
	return &Laureates{repository: repository, txManager: txManager}
}

// SaveDataset atomically applies a nonempty dataset snapshot.
func (s *Laureates) SaveDataset(ctx context.Context, records []models.Laureate) error {
	if len(records) == 0 {
		return models.ErrEmptyDataset
	}
	for i := range records {
		if err := normalize(&records[i]); err != nil {
			return fmt.Errorf("service.SaveDataset record %d: %w", i+1, err)
		}
	}
	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		for _, record := range records {
			if err := s.saveLaureate(txCtx, record); err != nil {
				return fmt.Errorf("service.SaveDataset laureate %q: %w", record.ID, err)
			}
		}
		return s.repository.SetLastUpdated(txCtx)
	})
	if err != nil {
		return fmt.Errorf("service.SaveDataset transaction: %w", err)
	}
	return nil
}

func (s *Laureates) saveLaureate(ctx context.Context, record models.Laureate) error {
	inserted, err := s.repository.Insert(ctx, record)
	if err != nil {
		return fmt.Errorf("service.saveLaureate insert details: %w", err)
	}
	if !inserted {
		if err := s.repository.Update(ctx, record); err != nil {
			return fmt.Errorf("service.saveLaureate update details: %w", err)
		}
	}
	if err := s.repository.ReplacePrizes(ctx, record); err != nil {
		return fmt.Errorf("service.saveLaureate save prizes: %w", err)
	}
	return nil
}

func normalize(record *models.Laureate) error {
	record.ID = strings.TrimSpace(record.ID)
	record.FirstName = strings.TrimSpace(record.FirstName)
	record.LastName = strings.TrimSpace(record.LastName)
	record.CountryCode = strings.ToUpper(strings.TrimSpace(record.CountryCode))
	if !idPattern.MatchString(record.ID) || record.FirstName == "" {
		return fmt.Errorf("service.normalize identifier and first name are required: %w", models.ErrInvalidLaureate)
	}
	if record.CountryCode != "" && !countryPattern.MatchString(record.CountryCode) {
		return fmt.Errorf("service.normalize country code must contain two letters: %w", models.ErrInvalidLaureate)
	}
	if record.Prizes == nil {
		record.Prizes = []models.Prize{}
	}
	for i := range record.Prizes {
		prize := &record.Prizes[i]
		prize.Category = strings.TrimSpace(prize.Category)
		if prize.Year < 1901 || prize.Year > 9999 || prize.Category == "" || prize.Share < 1 {
			return fmt.Errorf("service.normalize invalid prize %d: %w", i+1, models.ErrInvalidPrize)
		}
		if prize.Affiliations == nil {
			prize.Affiliations = []models.Affiliation{}
		}
	}
	return nil
}
