// Package nobel downloads and decodes the Nobel Prize API v1 dataset.
package nobel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"ris/internal/logger"
	"ris/internal/models"
)

// DefaultURL is the Nobel Prize API endpoint for laureates.
const DefaultURL = "https://api.nobelprize.org/v1/laureate.json"

const maxDatasetBytes = 16 << 20

type record struct {
	ID          string  `json:"id"`
	FirstName   string  `json:"firstname"`
	LastName    string  `json:"surname"`
	Gender      string  `json:"gender"`
	Born        string  `json:"born"`
	BornCity    string  `json:"bornCity"`
	BornCountry string  `json:"bornCountry"`
	CountryCode string  `json:"bornCountryCode"`
	Died        string  `json:"died"`
	DiedCity    string  `json:"diedCity"`
	DiedCountry string  `json:"diedCountry"`
	DiedCode    string  `json:"diedCountryCode"`
	Prizes      []prize `json:"prizes"`
}

type prize struct {
	Year              string               `json:"year"`
	Category          string               `json:"category"`
	Share             string               `json:"share"`
	Motivation        string               `json:"motivation"`
	OverallMotivation string               `json:"overallMotivation"`
	Affiliations      []models.Affiliation `json:"affiliations"`
}

// Download loads the v1 JSON snapshot and maps it to domain records.
func Download(ctx context.Context, client *http.Client, url string) ([]models.Laureate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nobel.Download build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nobel.Download request dataset: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nobel.Download unexpected HTTP status %s: %w", res.Status, models.ErrInvalid)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxDatasetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("nobel.Download read response: %w", err)
	}
	if len(body) > maxDatasetBytes {
		return nil, fmt.Errorf("nobel.Download dataset exceeds size limit: %w", models.ErrInvalid)
	}
	logger.FromContext(ctx).InfoContext(ctx, "dataset downloaded", "bytes", len(body))

	var dataset struct {
		Laureates []record `json:"laureates"`
	}
	if err := json.Unmarshal(body, &dataset); err != nil {
		return nil, fmt.Errorf("nobel.Download decode JSON: %w", err)
	}

	records := make([]models.Laureate, 0, len(dataset.Laureates))
	for _, item := range dataset.Laureates {
		record, err := item.toModel()
		if err != nil {
			return nil, fmt.Errorf("nobel.Download convert laureate %q: %w", item.ID, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func (r record) toModel() (models.Laureate, error) {
	laureate := models.Laureate{
		ID: r.ID, FirstName: r.FirstName, LastName: r.LastName, Gender: r.Gender,
		Born: r.Born, BornCity: r.BornCity, BornCountry: r.BornCountry, CountryCode: r.CountryCode,
		Died: r.Died, DiedCity: r.DiedCity, DiedCountry: r.DiedCountry, DiedCode: r.DiedCode,
		Prizes: make([]models.Prize, 0, len(r.Prizes)),
	}
	for _, sourcePrize := range r.Prizes {
		year, err := strconv.Atoi(sourcePrize.Year)
		if err != nil {
			return models.Laureate{}, fmt.Errorf("nobel.toModel parse award year: %w", err)
		}
		share, err := strconv.Atoi(sourcePrize.Share)
		if err != nil {
			return models.Laureate{}, fmt.Errorf("nobel.toModel parse award share: %w", err)
		}
		affiliations := make([]models.Affiliation, 0, len(sourcePrize.Affiliations))
		for _, affiliation := range sourcePrize.Affiliations {
			if affiliation != (models.Affiliation{}) {
				affiliations = append(affiliations, affiliation)
			}
		}
		laureate.Prizes = append(laureate.Prizes, models.Prize{
			Year: year, Category: sourcePrize.Category, Share: share, Motivation: sourcePrize.Motivation,
			OverallMotivation: sourcePrize.OverallMotivation, Affiliations: affiliations,
		})
	}
	return laureate, nil
}
