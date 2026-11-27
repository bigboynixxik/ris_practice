// Package nobel downloads and decodes the Nobel Prize API v1 dataset.
package nobel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"ris/internal/models"
	"ris/pkg/logger"
)

// DefaultURL is the Nobel Prize API endpoint for laureates.
const DefaultURL = "https://api.nobelprize.org/v1/laureate.json"

const maxDatasetBytes = 16 << 20

// ErrUnexpectedStatus indicates that the dataset endpoint did not return HTTP 200.
var ErrUnexpectedStatus = errors.New("unexpected dataset HTTP status")

// ErrDatasetTooLarge indicates that the input exceeds the supported size limit.
var ErrDatasetTooLarge = errors.New("dataset size limit exceeded")

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
	Year              string        `json:"year"`
	Category          string        `json:"category"`
	Share             string        `json:"share"`
	Motivation        string        `json:"motivation"`
	OverallMotivation string        `json:"overallMotivation"`
	Affiliations      []affiliation `json:"affiliations"`
}

type affiliation struct {
	models.Affiliation
}

func (a *affiliation) UnmarshalJSON(data []byte) error {
	// The v1 API represents a missing affiliation as an empty array entry.
	data = bytes.TrimSpace(data)
	if len(data) >= 2 && data[0] == '[' && len(bytes.TrimSpace(data[1:len(data)-1])) == 0 {
		return nil
	}
	return json.Unmarshal(data, &a.Affiliation)
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
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nobel.Download unexpected HTTP status %s: %w", res.Status, ErrUnexpectedStatus)
	}
	return decode(ctx, res.Body)
}

// ReadFile decodes a local file in the same format as the API response.
func ReadFile(ctx context.Context, path string) ([]models.Laureate, error) {
	file, err := os.Open(path) //nolint:gosec // The operator explicitly chooses the local input file.
	if err != nil {
		return nil, fmt.Errorf("nobel.ReadFile open JSON: %w", err)
	}
	defer func() { _ = file.Close() }()
	return decode(ctx, file)
}

func decode(ctx context.Context, source io.Reader) ([]models.Laureate, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("nobel.decode: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(source, maxDatasetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("nobel.decode read JSON: %w", err)
	}
	if len(body) > maxDatasetBytes {
		return nil, fmt.Errorf("nobel.decode dataset exceeds %d bytes: %w", maxDatasetBytes, ErrDatasetTooLarge)
	}
	logger.FromContext(ctx).InfoContext(ctx, "dataset read", "bytes", len(body))

	var dataset struct {
		Laureates []record `json:"laureates"`
	}
	if err := json.Unmarshal(body, &dataset); err != nil {
		return nil, fmt.Errorf("nobel.decode parse JSON: %w", err)
	}

	records := make([]models.Laureate, 0, len(dataset.Laureates))
	for _, item := range dataset.Laureates {
		record, err := item.toModel()
		if err != nil {
			return nil, fmt.Errorf("nobel.decode convert laureate %q: %w", item.ID, err)
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
			return models.Laureate{}, fmt.Errorf("nobel.toModel parse award year: %w: %w", models.ErrInvalidPrize, err)
		}
		share, err := strconv.Atoi(sourcePrize.Share)
		if err != nil {
			return models.Laureate{}, fmt.Errorf("nobel.toModel parse award share: %w: %w", models.ErrInvalidPrize, err)
		}
		affiliations := make([]models.Affiliation, 0, len(sourcePrize.Affiliations))
		for _, entry := range sourcePrize.Affiliations {
			if entry.Affiliation != (models.Affiliation{}) {
				affiliations = append(affiliations, entry.Affiliation)
			}
		}
		laureate.Prizes = append(laureate.Prizes, models.Prize{
			Year: year, Category: sourcePrize.Category, Share: share, Motivation: sourcePrize.Motivation,
			OverallMotivation: sourcePrize.OverallMotivation, Affiliations: affiliations,
		})
	}
	return laureate, nil
}
