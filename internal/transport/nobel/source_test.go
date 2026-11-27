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
	if count := len(records[1].Prizes[0].Affiliations); count != 0 {
		t.Fatalf("expected empty affiliation entries to be skipped, got %d", count)
	}
}

func TestDownloadRejectsHTTPError(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: http.NoBody}, nil
	})}
	_, err := Download(context.Background(), client, "https://example.test/missing")
	if !errors.Is(err, ErrUnexpectedStatus) {
		t.Fatalf("expected a wrapped HTTP status error, got %v", err)
	}
}

func TestReadFileUsesAPIFormat(t *testing.T) {
	records, err := ReadFile(context.Background(), "../../../examples/laureates.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || records[0].ID != "1" || records[0].Prizes[0].Year != 1901 {
		t.Fatalf("unexpected local records: %+v", records)
	}
	if _, err := ReadFile(context.Background(), "missing.json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected a missing file error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadFile(ctx, "../../../examples/laureates.json"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
