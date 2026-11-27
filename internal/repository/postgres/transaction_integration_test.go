//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ris/internal/app"
	"ris/internal/migrations"
	"ris/internal/models"
	"ris/internal/repository/postgres"
	"ris/internal/service"
	"ris/internal/transport/nobel"
	"ris/pkg/migrator"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func testDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("ris_test"),
		tcpostgres.WithUsername("ris"),
		tcpostgres.WithPassword("ris_test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start PostgreSQL test container: %v", err)
	}
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := migrator.New(db, migrations.Files).UpContext(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

func TestApplicationWaitsForShutdownWhileLoadingCommandsComplete(t *testing.T) {
	pool := testDatabase(t)
	logPath := filepath.Join(t.TempDir(), "ris.log")
	args := []string{"-dsn", pool.Config().ConnString(), "-log", logPath, "-file", "../../../examples/laureates.json"}
	application, err := app.New(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	var runErr error
	go func() {
		runErr = application.Run(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
			if runErr != nil {
				t.Errorf("application shutdown: %v", runErr)
			}
		case <-time.After(5 * time.Second):
			t.Error("application did not stop after cancellation")
		}
	})
	waitForApplicationReady(t, logPath, done)
	var count int
	if queryErr := pool.QueryRow(t.Context(), "SELECT count(*) FROM laureates").Scan(&count); queryErr != nil || count != 0 {
		t.Fatalf("startup unexpectedly loaded records: %d (%v)", count, queryErr)
	}
	command, err := app.New(append([]string{"-once"}, args...))
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Run(t.Context()); err != nil {
		t.Fatal(err)
	}
	if queryErr := pool.QueryRow(t.Context(), "SELECT count(*) FROM laureates").Scan(&count); queryErr != nil || count != 3 {
		t.Fatalf("loading command did not save records: %d (%v)", count, queryErr)
	}
	select {
	case <-done:
		t.Fatalf("application stopped after a loading command: %v", runErr)
	default:
	}
}

func waitForApplicationReady(t *testing.T, logPath string, done <-chan struct{}) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ticker.C:
			data, err := os.ReadFile(logPath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "application ready") {
				return
			}
		case <-done:
			t.Fatal("application exited before becoming ready")
		case <-timeout.C:
			t.Fatal("application did not become ready")
		}
	}
}

func TestServiceRollsBackAllRecordsAndTimestamp(t *testing.T) {
	ctx := context.Background()
	pool := testDatabase(t)
	repo := postgres.NewLaureatesRepo(pool)
	svc := service.NewLaureates(repo, postgres.NewTxManager(pool))
	records, err := nobel.ReadFile(ctx, "../../../examples/laureates.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveDataset(ctx, records); err != nil {
		t.Fatal(err)
	}
	var before time.Time
	if err := pool.QueryRow(ctx, "SELECT last_updated FROM dataset_metadata").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE prizes ADD CONSTRAINT reject_award CHECK (laureate_id <> '6') NOT VALID"); err != nil {
		t.Fatal(err)
	}
	changed := records[0]
	changed.FirstName = "Changed"
	newRecord := models.Laureate{ID: "new", FirstName: "New"}
	if err := svc.SaveDataset(ctx, []models.Laureate{changed, newRecord, records[2]}); err == nil {
		t.Fatal("expected the last record to fail")
	}
	var firstName string
	var count int
	var after time.Time
	if err := pool.QueryRow(ctx, "SELECT first_name, (SELECT count(*) FROM laureates), (SELECT last_updated FROM dataset_metadata) FROM laureates WHERE id = '1'").Scan(&firstName, &count, &after); err != nil {
		t.Fatal(err)
	}
	if firstName != records[0].FirstName || count != 3 || !before.Equal(after) {
		t.Fatalf("partial changes survived rollback: %s, %d, %v", firstName, count, after)
	}
	var prizes, affiliations int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM prizes), (SELECT count(*) FROM affiliations)").Scan(&prizes, &affiliations); err != nil {
		t.Fatal(err)
	}
	if prizes != 3 || affiliations != 1 {
		t.Fatalf("partial award changes survived rollback: %d, %d", prizes, affiliations)
	}
}

func TestConcurrentLoadsChooseInsertAndUpdateSafely(t *testing.T) {
	ctx := context.Background()
	pool := testDatabase(t)
	svc := service.NewLaureates(postgres.NewLaureatesRepo(pool), postgres.NewTxManager(pool))
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Go(func() {
			records, err := nobel.ReadFile(ctx, "../../../examples/laureates.json")
			if err != nil {
				t.Error(err)
				return
			}
			if err := svc.SaveDataset(ctx, records); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM laureates").Scan(&count); err != nil || count != 3 {
		t.Fatalf("unexpected concurrent load result: %d (%v)", count, err)
	}
	records, err := nobel.ReadFile(ctx, "../../../examples/laureates.json")
	if err != nil {
		t.Fatal(err)
	}
	records[0].FirstName = "Updated"
	records[0].Prizes[0].Share = 2
	records[0].Prizes[0].Affiliations[0].City = "Updated city"
	if err := svc.SaveDataset(ctx, records); err != nil {
		t.Fatal(err)
	}
	var name, city string
	var share int
	if err := pool.QueryRow(ctx, "SELECT l.first_name, p.share, a.city FROM laureates l JOIN prizes p ON p.laureate_id = l.id JOIN affiliations a ON a.prize_id = p.id WHERE l.id = '1'").Scan(&name, &share, &city); err != nil {
		t.Fatal(err)
	}
	if name != "Updated" || share != 2 || city != "Updated city" {
		t.Fatalf("details or awards were not updated: %s, %d, %s", name, share, city)
	}
}

func TestTxManagerRollsBackAfterCancellation(t *testing.T) {
	pool := testDatabase(t)
	repo := postgres.NewLaureatesRepo(pool)
	tm := postgres.NewTxManager(pool)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := tm.WithTx(ctx, func(txCtx context.Context) error {
		if _, err := repo.Insert(txCtx, models.Laureate{ID: "canceled", FirstName: "Canceled"}); err != nil {
			return err
		}
		cancel()
		return txCtx.Err()
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	found, err := repo.Exists(context.Background(), "canceled")
	if err != nil || found {
		t.Fatalf("canceled transaction persisted a record: %v (%v)", found, err)
	}
}
