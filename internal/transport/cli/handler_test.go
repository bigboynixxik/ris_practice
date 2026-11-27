package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ris/internal/models"
	"ris/internal/transport/nobel"
	"ris/pkg/logger"
)

type saveFunc func(context.Context, []models.Laureate) error

func (f saveFunc) SaveDataset(ctx context.Context, records []models.Laureate) error {
	return f(ctx, records)
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type logEntry struct {
	Level     string `json:"level"`
	Message   string `json:"msg"`
	Transport string `json:"transport"`
	Error     string `json:"error"`
	Laureates int    `json:"laureates"`
}

func readLogs(t *testing.T, data []byte) []logEntry {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	var entries []logEntry
	for {
		var entry logEntry
		if err := decoder.Decode(&entry); errors.Is(err, io.EOF) {
			return entries
		} else if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
}

func TestLoadLogsErrorsFromSourceAndService(t *testing.T) {
	const dataset = `{"laureates":[{"id":"1","firstname":"Test","prizes":[]}]}`
	databaseErr := errors.New("database unavailable")
	cases := []struct {
		name        string
		status      int
		body        string
		file        string
		serviceErr  error
		cause       error
		wantCalls   int
		wantMessage string
	}{
		{name: "service", status: http.StatusOK, body: dataset,
			serviceErr: fmt.Errorf("service.SaveDataset transaction: %w", databaseErr),
			cause:      databaseErr, wantCalls: 1, wantMessage: "service.SaveDataset transaction"},
		{name: "download", status: http.StatusNotFound, cause: nobel.ErrUnexpectedStatus,
			wantMessage: "404 Not Found"},
		{name: "decode", status: http.StatusOK, body: "{", wantMessage: "nobel.decode parse JSON"},
		{name: "file", file: "missing.json", cause: os.ErrNotExist, wantMessage: "nobel.ReadFile open JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			ctx := logger.WithContext(t.Context(), logger.NewForEnv("dev", &output, "ris"))
			client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status,
					Status: fmt.Sprintf("%d %s", tc.status, http.StatusText(tc.status)),
					Body:   io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			calls := 0
			saver := saveFunc(func(context.Context, []models.Laureate) error {
				calls++
				return tc.serviceErr
			})
			file := ""
			if tc.file != "" {
				file = filepath.Join(t.TempDir(), tc.file)
			}
			_, err := NewHandler(client, saver, "https://example.test/laureates.json", file).Load(ctx)
			if err == nil || !strings.Contains(err.Error(), tc.wantMessage) {
				t.Fatalf("expected %q, got %v", tc.wantMessage, err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("original error was lost: %v", err)
			}
			if calls != tc.wantCalls {
				t.Fatalf("unexpected service calls: %d", calls)
			}
			var failures int
			for _, entry := range readLogs(t, output.Bytes()) {
				if entry.Message == "dataset loading finished" {
					t.Fatal("failure was logged as a successful load")
				}
				if entry.Level == "ERROR" {
					failures++
					if entry.Message != "dataset loading failed" || entry.Transport != "cli" || entry.Error != err.Error() {
						t.Fatalf("incomplete transport error log: %+v", entry)
					}
				}
			}
			if failures != 1 {
				t.Fatalf("expected one error log, got %d", failures)
			}
		})
	}
}

func TestLoadLogsSuccessfulLocalFile(t *testing.T) {
	var output bytes.Buffer
	ctx := logger.WithContext(t.Context(), logger.NewForEnv("dev", &output, "ris"))
	var saved int
	saver := saveFunc(func(_ context.Context, records []models.Laureate) error {
		saved = len(records)
		return nil
	})
	count, err := NewHandler(http.DefaultClient, saver, "", "../../../examples/laureates.json").Load(ctx)
	if err != nil || count != 3 || saved != 3 {
		t.Fatalf("unexpected local load result: count=%d saved=%d error=%v", count, saved, err)
	}
	var finished int
	for _, entry := range readLogs(t, output.Bytes()) {
		if entry.Level == "ERROR" {
			t.Fatalf("successful load logged an error: %+v", entry)
		}
		if entry.Message == "dataset loading finished" && entry.Laureates == 3 && entry.Transport == "cli" {
			finished++
		}
	}
	if finished != 1 {
		t.Fatalf("expected one completion log, got %d", finished)
	}
}

func TestLoadLogsCancellationWithoutAnError(t *testing.T) {
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(logger.WithContext(t.Context(), logger.NewForEnv("dev", &output, "ris")))
	cancel()
	saver := saveFunc(func(context.Context, []models.Laureate) error {
		t.Fatal("service must not run after input cancellation")
		return nil
	})
	_, err := NewHandler(http.DefaultClient, saver, "", "../../../examples/laureates.json").Load(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	var canceled int
	for _, entry := range readLogs(t, output.Bytes()) {
		if entry.Level == "ERROR" || entry.Message == "dataset loading finished" {
			t.Fatalf("unexpected cancellation log: %+v", entry)
		}
		if entry.Message == "dataset loading canceled" && entry.Level == "INFO" {
			canceled++
		}
	}
	if canceled != 1 {
		t.Fatalf("expected one cancellation log, got %d", canceled)
	}
}
