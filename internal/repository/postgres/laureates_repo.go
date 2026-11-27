// Package postgres persists the dataset in PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"ris/internal/models"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const datasetID = 1

// LaureatesRepo stores laureates and their awards in PostgreSQL.
type LaureatesRepo struct {
	pool *pgxpool.Pool
	sq   sq.StatementBuilderType
}

// NewLaureatesRepo creates a repository with a PostgreSQL Squirrel builder.
func NewLaureatesRepo(pool *pgxpool.Pool) *LaureatesRepo {
	return &LaureatesRepo{
		pool: pool,
		sq:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

// Exists reports whether a laureate identifier is already stored.
func (r *LaureatesRepo) Exists(ctx context.Context, id string) (bool, error) {
	query, args, err := r.sq.Select("id").From("laureates").
		Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return false, fmt.Errorf("laureates_repo.Exists sql: %w", err)
	}
	var found string
	if err := Querier(ctx, r.pool).QueryRow(ctx, query, args...).Scan(&found); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("laureates_repo.Exists scan: %w", err)
	}
	return true, nil
}

// Insert creates a laureate and returns false when its identifier already exists.
func (r *LaureatesRepo) Insert(ctx context.Context, record models.Laureate) (bool, error) {
	query, args, err := r.sq.Insert("laureates").
		Columns("id", "first_name", "last_name", "gender", "born", "born_city", "born_country",
			"country_code", "died", "died_city", "died_country", "died_country_code").
		Values(record.ID, record.FirstName, record.LastName, record.Gender, record.Born,
			record.BornCity, record.BornCountry, record.CountryCode, record.Died,
			record.DiedCity, record.DiedCountry, record.DiedCode).
		Suffix("ON CONFLICT (id) DO NOTHING").
		ToSql()
	if err != nil {
		return false, fmt.Errorf("laureates_repo.Insert sql: %w", err)
	}
	result, err := Querier(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("laureates_repo.Insert exec: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

// Update changes the identifying details of an existing laureate.
func (r *LaureatesRepo) Update(ctx context.Context, record models.Laureate) error {
	query, args, err := r.sq.Update("laureates").
		Set("first_name", record.FirstName).
		Set("last_name", record.LastName).
		Set("gender", record.Gender).
		Set("born", record.Born).
		Set("born_city", record.BornCity).
		Set("born_country", record.BornCountry).
		Set("country_code", record.CountryCode).
		Set("died", record.Died).
		Set("died_city", record.DiedCity).
		Set("died_country", record.DiedCountry).
		Set("died_country_code", record.DiedCode).
		Where(sq.Eq{"id": record.ID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("laureates_repo.Update sql: %w", err)
	}
	result, err := Querier(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("laureates_repo.Update exec: %w", err)
	}
	if result.RowsAffected() != 1 {
		return models.ErrLaureateNotFound
	}
	return nil
}

// ReplacePrizes stores the current awards and affiliations of a laureate.
func (r *LaureatesRepo) ReplacePrizes(ctx context.Context, record models.Laureate) error {
	query, args, err := r.sq.Delete("prizes").Where(sq.Eq{"laureate_id": record.ID}).ToSql()
	if err != nil {
		return fmt.Errorf("laureates_repo.ReplacePrizes delete sql: %w", err)
	}
	if _, err := Querier(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("laureates_repo.ReplacePrizes delete: %w", err)
	}
	for _, prize := range record.Prizes {
		if err := r.insertPrize(ctx, record.ID, prize); err != nil {
			return fmt.Errorf("laureates_repo.ReplacePrizes insert: %w", err)
		}
	}
	return nil
}

func (r *LaureatesRepo) insertPrize(ctx context.Context, laureateID string, prize models.Prize) error {
	query, args, err := r.sq.Insert("prizes").
		Columns("laureate_id", "year", "category", "share", "motivation", "overall_motivation").
		Values(laureateID, prize.Year, prize.Category, prize.Share, prize.Motivation, prize.OverallMotivation).
		Suffix("RETURNING id").ToSql()
	if err != nil {
		return fmt.Errorf("laureates_repo.insertPrize prize sql: %w", err)
	}
	var prizeID int64
	if scanErr := Querier(ctx, r.pool).QueryRow(ctx, query, args...).Scan(&prizeID); scanErr != nil {
		return fmt.Errorf("laureates_repo.insertPrize scan id: %w", scanErr)
	}

	if len(prize.Affiliations) == 0 {
		return nil
	}
	affiliations := r.sq.Insert("affiliations").Columns("prize_id", "name", "city", "country")
	for _, affiliation := range prize.Affiliations {
		affiliations = affiliations.Values(prizeID, affiliation.Name, affiliation.City, affiliation.Country)
	}
	query, args, err = affiliations.ToSql()
	if err != nil {
		return fmt.Errorf("laureates_repo.insertPrize affiliations sql: %w", err)
	}
	if _, err := Querier(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("laureates_repo.insertPrize insert affiliations: %w", err)
	}
	return nil
}

// SetLastUpdated records the time of a fully successful dataset load.
func (r *LaureatesRepo) SetLastUpdated(ctx context.Context) error {
	query, args, err := r.sq.Update("dataset_metadata").
		Set("last_updated", sq.Expr("clock_timestamp()")).
		Where(sq.Eq{"id": datasetID}).ToSql()
	if err != nil {
		return fmt.Errorf("laureates_repo.SetLastUpdated sql: %w", err)
	}
	result, err := Querier(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("laureates_repo.SetLastUpdated exec: %w", err)
	}
	if result.RowsAffected() != 1 {
		return models.ErrDatasetMetadataMissing
	}
	return nil
}
