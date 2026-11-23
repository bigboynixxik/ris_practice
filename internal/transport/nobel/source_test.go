package nobel

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"ris/internal/models"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDownloadMapsNestedDataset(t *testing.T) {
	data, err := os.ReadFile("../../../examples/laureates.json")
	if err != nil {
		t.Fatal(err)
	}
	const url = "https://example.test/laureates.json"
	client := &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != url || req.Method != http.MethodGet || req.Header.Get("Accept") != "application/json" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
	})}

	records, err := Download(context.Background(), client, url)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	if !slices.Equal(ids, []string{"1", "2", "6"}) {
		t.Fatalf("unexpected IDs: %v", ids)
	}
	record := records[0]
	if record.CountryCode != "DE" || len(record.Prizes) != 1 {
		t.Fatalf("unexpected first record: %+v", record)
	}
	prize := record.Prizes[0]
	if prize.Year != 1901 || prize.Share != 1 || len(prize.Affiliations) != 1 || prize.Affiliations[0].City != "Munich" {
		t.Fatalf("unexpected prize: %+v", prize)
	}
}

func TestDownloadRejectsHTTPError(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: http.NoBody}, nil
	})}
	_, err := Download(context.Background(), client, "https://example.test/missing")
	if !errors.Is(err, models.ErrInvalid) {
		t.Fatalf("expected wrapped invalid input error, got %v", err)
	}
}
