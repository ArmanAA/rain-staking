package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArmanAA/rain-staking/gen/sqlc"
	"github.com/ArmanAA/rain-staking/internal/domain"
)

// BalanceRepo implements port.BalanceRepository using PostgreSQL.
type BalanceRepo struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewBalanceRepo(pool *pgxpool.Pool) *BalanceRepo {
	return &BalanceRepo{
		pool:    pool,
		queries: sqlcgen.New(pool),
	}
}

func (r *BalanceRepo) GetByCustomerAndAsset(ctx context.Context, customerID, asset string) (*domain.Balance, error) {
	row, err := r.queries.GetBalanceByCustomerAndAsset(ctx, sqlcgen.GetBalanceByCustomerAndAssetParams{
		CustomerID: toUUID(customerID),
		Asset:      asset,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBalanceNotFound
		}
		return nil, err
	}
	return balanceFromRow(row), nil
}

func (r *BalanceRepo) ListByCustomerID(ctx context.Context, customerID string) ([]*domain.Balance, error) {
	rows, err := r.queries.ListBalancesByCustomerID(ctx, toUUID(customerID))
	if err != nil {
		return nil, err
	}
	balances := make([]*domain.Balance, len(rows))
	for i, row := range rows {
		balances[i] = balanceFromRow(row)
	}
	return balances, nil
}

func (r *BalanceRepo) Upsert(ctx context.Context, balance *domain.Balance) error {
	return r.queries.UpsertBalance(ctx, sqlcgen.UpsertBalanceParams{
		ID:         toUUID(balance.ID),
		CustomerID: toUUID(balance.CustomerID),
		Asset:      balance.Asset,
		Available:  toNumeric(balance.Available),
		Staked:     toNumeric(balance.Staked),
		Pending:    toNumeric(balance.Pending),
		Version:    balance.Version,
		CreatedAt:  toTimestamptz(balance.CreatedAt),
		UpdatedAt:  toTimestamptz(balance.UpdatedAt),
	})
}

func (r *BalanceRepo) Update(ctx context.Context, balance *domain.Balance) error {
	oldVersion := balance.Version - 1
	rowsAffected, err := r.queries.UpdateBalance(ctx, sqlcgen.UpdateBalanceParams{
		Available: toNumeric(balance.Available),
		Staked:    toNumeric(balance.Staked),
		Pending:   toNumeric(balance.Pending),
		Version:   balance.Version,
		UpdatedAt: toTimestamptz(balance.UpdatedAt),
		ID:        toUUID(balance.ID),
		Version_2: oldVersion,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func balanceFromRow(row sqlcgen.Balance) *domain.Balance {
	return &domain.Balance{
		ID:         fromUUID(row.ID),
		CustomerID: fromUUID(row.CustomerID),
		Asset:      row.Asset,
		Available:  fromNumeric(row.Available),
		Staked:     fromNumeric(row.Staked),
		Pending:    fromNumeric(row.Pending),
		Version:    row.Version,
		CreatedAt:  fromTimestamptz(row.CreatedAt),
		UpdatedAt:  fromTimestamptz(row.UpdatedAt),
	}
}
