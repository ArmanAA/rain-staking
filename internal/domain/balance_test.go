package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBalance(t *testing.T) {
	t.Run("creates balance with zero values", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")

		assert.NotEmpty(t, balance.ID)
		assert.Equal(t, "cust-1", balance.CustomerID)
		assert.Equal(t, "ETH", balance.Asset)
		assert.True(t, balance.Available.IsZero())
		assert.True(t, balance.Staked.IsZero())
		assert.True(t, balance.Pending.IsZero())
		assert.Equal(t, int64(1), balance.Version)
	})
}

func TestBalanceHold(t *testing.T) {
	t.Run("moves available to pending", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(100.0)

		err := balance.Hold(decimal.NewFromFloat(32.0))

		require.NoError(t, err)
		assert.True(t, balance.Available.Equal(decimal.NewFromFloat(68.0)))
		assert.True(t, balance.Pending.Equal(decimal.NewFromFloat(32.0)))
	})

	t.Run("rejects hold exceeding available", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(10.0)

		err := balance.Hold(decimal.NewFromFloat(32.0))

		assert.ErrorIs(t, err, ErrInsufficientBalance)
		assert.True(t, balance.Available.Equal(decimal.NewFromFloat(10.0)), "balance unchanged on error")
		assert.True(t, balance.Pending.IsZero(), "pending unchanged on error")
	})

	t.Run("rejects zero amount", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(100.0)

		err := balance.Hold(decimal.Zero)
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})

	t.Run("rejects negative amount", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(100.0)

		err := balance.Hold(decimal.NewFromFloat(-5.0))
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})
}

func TestBalanceConfirmStake(t *testing.T) {
	t.Run("moves pending to staked", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(68.0)
		balance.Pending = decimal.NewFromFloat(32.0)

		err := balance.ConfirmStake(decimal.NewFromFloat(32.0))

		require.NoError(t, err)
		assert.True(t, balance.Pending.IsZero())
		assert.True(t, balance.Staked.Equal(decimal.NewFromFloat(32.0)))
	})

	t.Run("rejects if pending insufficient", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Pending = decimal.NewFromFloat(16.0)

		err := balance.ConfirmStake(decimal.NewFromFloat(32.0))

		assert.ErrorIs(t, err, ErrInsufficientBalance)
	})
}

func TestBalanceReleaseHold(t *testing.T) {
	t.Run("returns pending back to available", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(68.0)
		balance.Pending = decimal.NewFromFloat(32.0)

		err := balance.ReleaseHold(decimal.NewFromFloat(32.0))

		require.NoError(t, err)
		assert.True(t, balance.Available.Equal(decimal.NewFromFloat(100.0)))
		assert.True(t, balance.Pending.IsZero())
	})
}

func TestBalanceCompleteUnstake(t *testing.T) {
	t.Run("moves staked back to available", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(68.0)
		balance.Staked = decimal.NewFromFloat(32.0)

		err := balance.CompleteUnstake(decimal.NewFromFloat(32.0))

		require.NoError(t, err)
		assert.True(t, balance.Available.Equal(decimal.NewFromFloat(100.0)))
		assert.True(t, balance.Staked.IsZero())
	})

	t.Run("rejects if staked insufficient", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Staked = decimal.NewFromFloat(16.0)

		err := balance.CompleteUnstake(decimal.NewFromFloat(32.0))

		assert.ErrorIs(t, err, ErrInsufficientBalance)
	})
}

func TestBalanceAddReward(t *testing.T) {
	t.Run("adds reward to available", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")
		balance.Available = decimal.NewFromFloat(68.0)

		err := balance.AddReward(decimal.NewFromFloat(0.5))

		require.NoError(t, err)
		assert.True(t, balance.Available.Equal(decimal.NewFromFloat(68.5)))
	})

	t.Run("rejects zero reward", func(t *testing.T) {
		balance := NewBalance("cust-1", "ETH")

		err := balance.AddReward(decimal.Zero)
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})
}
