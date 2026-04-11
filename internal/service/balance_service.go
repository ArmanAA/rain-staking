package service

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/ArmanAA/rain-staking/internal/domain"
	"github.com/ArmanAA/rain-staking/internal/port"
)

// BalanceService handles balance queries and mutations.
type BalanceService struct {
	balanceRepo port.BalanceRepository
	publisher   port.EventPublisher
}

func NewBalanceService(balanceRepo port.BalanceRepository, publisher port.EventPublisher) *BalanceService {
	return &BalanceService{
		balanceRepo: balanceRepo,
		publisher:   publisher,
	}
}

// GetBalance retrieves a customer's balance for a specific asset.
func (s *BalanceService) GetBalance(ctx context.Context, customerID, asset string) (*domain.Balance, error) {
	return s.balanceRepo.GetByCustomerAndAsset(ctx, customerID, asset)
}

// ListBalances retrieves all balances for a customer.
func (s *BalanceService) ListBalances(ctx context.Context, customerID string) ([]*domain.Balance, error) {
	return s.balanceRepo.ListByCustomerID(ctx, customerID)
}

// CreateOrUpdateBalance creates or updates a customer balance (used for seeding/deposits).
func (s *BalanceService) CreateOrUpdateBalance(ctx context.Context, customerID, asset string, available decimal.Decimal) (*domain.Balance, error) {
	balance, err := s.balanceRepo.GetByCustomerAndAsset(ctx, customerID, asset)
	if err != nil {
		if err == domain.ErrBalanceNotFound {
			balance = domain.NewBalance(customerID, asset)
			balance.Available = available
			if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
				return nil, err
			}

			s.publisher.Publish(ctx, domain.NewAuditEvent(
				"balance", balance.ID, "balance.created", customerID,
				map[string]any{"asset": asset, "available": available.String()},
			))

			return balance, nil
		}
		return nil, err
	}

	balance.Available = available
	balance.Version++
	if err := s.balanceRepo.Update(ctx, balance); err != nil {
		return nil, err
	}

	return balance, nil
}
