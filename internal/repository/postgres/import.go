// Package postgres persists the dataset in PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"ris/internal/models"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var sql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var laureateColumns = []string{
	"id", "first_name", "last_name", "gender", "born", "born_city", "born_country",
	"country_code", "died", "died_city", "died_country", "died_country_code",
}

// Repository stores laureates and their awards in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New constructs a dataset repository.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Import upserts an entire snapshot in one transaction.
func (r *Repository) Import(ctx context.Context, records []models.Laureate) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var datasetID int
		query := sql.Select("id").From("dataset_metadata").Where(sq.Eq{"id": 1}).Suffix("FOR UPDATE")
		if err := scan(ctx, tx, query, &datasetID); err != nil {
			return fmt.Errorf("postgres.Import lock dataset: %w", err)
		}

		for _, record := range records {
			if err := upsertLaureate(ctx, tx, record); err != nil {
				return fmt.Errorf("postgres.Import save record %q: %w", record.ID, err)
			}
		}

		queryUpdate := sql.Update("dataset_metadata").Set("last_updated", sq.Expr("clock_timestamp()")).Where(sq.Eq{"id": datasetID})
		if err := exec(ctx, tx, queryUpdate); err != nil {
			return fmt.Errorf("postgres.Import update dataset timestamp: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("postgres.Import execute dataset transaction: %w", err)
	}
	return nil
}

func upsertLaureate(ctx context.Context, tx pgx.Tx, record models.Laureate) error {
	updates := "ON CONFLICT (id) DO UPDATE SET "
	for i, column := range laureateColumns[1:] {
		if i > 0 {
			updates += ", "
		}
		updates += column + " = EXCLUDED." + column
	}
	if err := exec(ctx, tx, insertLaureate(record).Suffix(updates)); err != nil {
		return fmt.Errorf("postgres.upsertLaureate upsert row: %w", err)
	}

	removePrizes := sql.Delete("prizes").Where(sq.Eq{"laureate_id": record.ID})
	if err := exec(ctx, tx, removePrizes); err != nil {
		return fmt.Errorf("postgres.upsertLaureate replace existing prizes: %w", err)
	}
	if err := insertPrizes(ctx, tx, record); err != nil {
		return fmt.Errorf("postgres.upsertLaureate save prizes: %w", err)
	}
	return nil
}

func insertLaureate(record models.Laureate) sq.InsertBuilder {
	return sql.Insert("laureates").Columns(laureateColumns...).Values(
		record.ID, record.FirstName, record.LastName, record.Gender, record.Born,
		record.BornCity, record.BornCountry, record.CountryCode, record.Died,
		record.DiedCity, record.DiedCountry, record.DiedCode,
	)
}

func insertPrizes(ctx context.Context, tx pgx.Tx, record models.Laureate) error {
	for _, prize := range record.Prizes {
		query := sql.Insert("prizes").
			Columns("laureate_id", "year", "category", "share", "motivation", "overall_motivation").
			Values(record.ID, prize.Year, prize.Category, prize.Share, prize.Motivation, prize.OverallMotivation).
			Suffix("RETURNING id")
		var prizeID int64
		if err := scan(ctx, tx, query, &prizeID); err != nil {
			return fmt.Errorf("postgres.insertPrizes insert award: %w", err)
		}

		if len(prize.Affiliations) == 0 {
			continue
		}
		affiliations := sql.Insert("affiliations").Columns("prize_id", "name", "city", "country")
		for _, affiliation := range prize.Affiliations {
			affiliations = affiliations.Values(prizeID, affiliation.Name, affiliation.City, affiliation.Country)
		}
		if err := exec(ctx, tx, affiliations); err != nil {
			return fmt.Errorf("postgres.insertPrizes insert affiliations: %w", err)
		}
	}
	return nil
}

func scan(ctx context.Context, tx pgx.Tx, query sq.Sqlizer, dest ...any) error {
	statement, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("postgres.scan build query: %w", err)
	}
	if err := tx.QueryRow(ctx, statement, args...).Scan(dest...); err != nil {
		return fmt.Errorf("postgres.scan read row: %w", err)
	}
	return nil
}

func exec(ctx context.Context, tx pgx.Tx, query sq.Sqlizer) error {
	statement, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("postgres.exec build query: %w", err)
	}
	if _, err := tx.Exec(ctx, statement, args...); err != nil {
		return fmt.Errorf("postgres.exec execute query: %w", err)
	}
	return nil
}
