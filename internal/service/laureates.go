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
	Import(context.Context, []models.Laureate) error
}

// Laureates validates inputs and coordinates dataset operations.
type Laureates struct {
	repository Repository
}

// NewLaureates constructs the dataset service.
func NewLaureates(repository Repository) *Laureates {
	return &Laureates{repository: repository}
}

// Import atomically applies a nonempty dataset snapshot.
func (s *Laureates) Import(ctx context.Context, records []models.Laureate) error {
	if len(records) == 0 {
		return fmt.Errorf("service.Import empty dataset: %w", models.ErrInvalid)
	}
	for i := range records {
		if err := normalize(&records[i]); err != nil {
			return fmt.Errorf("service.Import record %d: %w", i+1, err)
		}
	}
	if err := s.repository.Import(ctx, records); err != nil {
		return fmt.Errorf("service.Import save dataset: %w", err)
	}
	return nil
}

func normalize(record *models.Laureate) error {
	record.ID = strings.TrimSpace(record.ID)
	record.FirstName = strings.TrimSpace(record.FirstName)
	record.LastName = strings.TrimSpace(record.LastName)
	record.CountryCode = strings.ToUpper(strings.TrimSpace(record.CountryCode))
	if !idPattern.MatchString(record.ID) || record.FirstName == "" {
		return fmt.Errorf("service.normalize identifier and first name are required: %w", models.ErrInvalid)
	}
	if record.CountryCode != "" && !countryPattern.MatchString(record.CountryCode) {
		return fmt.Errorf("service.normalize country code must contain two letters: %w", models.ErrInvalid)
	}
	if record.Prizes == nil {
		record.Prizes = []models.Prize{}
	}
	for i := range record.Prizes {
		prize := &record.Prizes[i]
		prize.Category = strings.TrimSpace(prize.Category)
		if prize.Year < 1901 || prize.Year > 9999 || prize.Category == "" || prize.Share < 1 {
			return fmt.Errorf("service.normalize invalid prize %d: %w", i+1, models.ErrInvalid)
		}
		if prize.Affiliations == nil {
			prize.Affiliations = []models.Affiliation{}
		}
	}
	return nil
}
