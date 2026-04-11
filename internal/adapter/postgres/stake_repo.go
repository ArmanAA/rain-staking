package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArmanAA/rain-staking/gen/sqlc"
	"github.com/ArmanAA/rain-staking/internal/domain"
)

// StakeRepo implements port.StakeRepository using PostgreSQL.
type StakeRepo struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

func NewStakeRepo(pool *pgxpool.Pool) *StakeRepo {
	return &StakeRepo{
		pool:    pool,
		queries: sqlcgen.New(pool),
	}
}

func (r *StakeRepo) Create(ctx context.Context, stake *domain.Stake) error {
	return r.queries.CreateStake(ctx, sqlcgen.CreateStakeParams{
		ID:             toUUID(stake.ID),
		CustomerID:     toUUID(stake.CustomerID),
		Asset:          stake.Asset,
		Amount:         toNumeric(stake.Amount),
		State:          string(stake.State),
		ProviderRef:    toText(stake.ProviderRef),
		Validator:      toText(stake.Validator),
		IdempotencyKey: toText(stake.IdempotencyKey),
		FailureReason:  toText(stake.FailureReason),
		Version:        stake.Version,
		CreatedAt:      toTimestamptz(stake.CreatedAt),
		UpdatedAt:      toTimestamptz(stake.UpdatedAt),
	})
}

func (r *StakeRepo) GetByID(ctx context.Context, id string) (*domain.Stake, error) {
	row, err := r.queries.GetStakeByID(ctx, toUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStakeNotFound
		}
		return nil, err
	}
	return stakeFromRow(row), nil
}

func (r *StakeRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Stake, error) {
	row, err := r.queries.GetStakeByIdempotencyKey(ctx, toText(key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Not found is not an error for idempotency checks
		}
		return nil, err
	}
	return stakeFromRow(row), nil
}

func (r *StakeRepo) ListByCustomerID(ctx context.Context, customerID string, state *domain.StakeState, limit, offset int) ([]*domain.Stake, error) {
	if state != nil {
		rows, err := r.queries.ListStakesByCustomerIDAndState(ctx, sqlcgen.ListStakesByCustomerIDAndStateParams{
			CustomerID: toUUID(customerID),
			State:      string(*state),
			Limit:      int32(limit),
			Offset:     int32(offset),
		})
		if err != nil {
			return nil, err
		}
		return stakesFromRows(rows), nil
	}

	rows, err := r.queries.ListStakesByCustomerID(ctx, sqlcgen.ListStakesByCustomerIDParams{
		CustomerID: toUUID(customerID),
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, err
	}
	return stakesFromRows(rows), nil
}

func (r *StakeRepo) Update(ctx context.Context, stake *domain.Stake) error {
	oldVersion := stake.Version - 1
	rowsAffected, err := r.queries.UpdateStake(ctx, sqlcgen.UpdateStakeParams{
		State:         string(stake.State),
		ProviderRef:   toText(stake.ProviderRef),
		Validator:     toText(stake.Validator),
		FailureReason: toText(stake.FailureReason),
		Version:       stake.Version,
		UpdatedAt:     toTimestamptz(stake.UpdatedAt),
		ID:            toUUID(stake.ID),
		Version_2:     oldVersion,
	})
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *StakeRepo) ListByState(ctx context.Context, state domain.StakeState, limit int) ([]*domain.Stake, error) {
	rows, err := r.queries.ListStakesByState(ctx, sqlcgen.ListStakesByStateParams{
		State: string(state),
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return stakesFromRows(rows), nil
}

func stakeFromRow(row sqlcgen.Stake) *domain.Stake {
	return &domain.Stake{
		ID:             fromUUID(row.ID),
		CustomerID:     fromUUID(row.CustomerID),
		Asset:          row.Asset,
		Amount:         fromNumeric(row.Amount),
		State:          domain.StakeState(row.State),
		ProviderRef:    fromText(row.ProviderRef),
		Validator:      fromText(row.Validator),
		IdempotencyKey: fromText(row.IdempotencyKey),
		FailureReason:  fromText(row.FailureReason),
		Version:        row.Version,
		CreatedAt:      fromTimestamptz(row.CreatedAt),
		UpdatedAt:      fromTimestamptz(row.UpdatedAt),
	}
}

func stakesFromRows(rows []sqlcgen.Stake) []*domain.Stake {
	stakes := make([]*domain.Stake, len(rows))
	for i, row := range rows {
		stakes[i] = stakeFromRow(row)
	}
	return stakes
}
