package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStake(t *testing.T) {
	t.Run("valid stake creation", func(t *testing.T) {
		amount := decimal.NewFromFloat(32.0)
		stake, err := NewStake("cust-1", "ETH", amount, "idem-1")

		require.NoError(t, err)
		assert.NotEmpty(t, stake.ID)
		assert.Equal(t, "cust-1", stake.CustomerID)
		assert.Equal(t, "ETH", stake.Asset)
		assert.True(t, stake.Amount.Equal(amount))
		assert.Equal(t, StakeStatePending, stake.State)
		assert.Equal(t, "idem-1", stake.IdempotencyKey)
		assert.Equal(t, int64(1), stake.Version)
		assert.NotZero(t, stake.CreatedAt)
		assert.NotZero(t, stake.UpdatedAt)
	})

	t.Run("zero amount rejected", func(t *testing.T) {
		_, err := NewStake("cust-1", "ETH", decimal.Zero, "idem-1")
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		_, err := NewStake("cust-1", "ETH", decimal.NewFromFloat(-1.0), "idem-1")
		assert.ErrorIs(t, err, ErrInvalidAmount)
	})
}

func TestStakeStateTransitions(t *testing.T) {
	tests := []struct {
		name        string
		fromState   StakeState
		transition  string
		expectState StakeState
		expectErr   error
	}{
		// Valid transitions
		{"pending to delegating", StakeStatePending, "Delegate", StakeStateDelegating, nil},
		{"delegating to active", StakeStateDelegating, "Activate", StakeStateActive, nil},
		{"active to unstaking", StakeStateActive, "Unstake", StakeStateUnstaking, nil},
		{"unstaking to withdrawn", StakeStateUnstaking, "Withdraw", StakeStateWithdrawn, nil},

		// Valid failure transitions
		{"pending to failed", StakeStatePending, "Fail", StakeStateFailed, nil},
		{"delegating to failed", StakeStateDelegating, "Fail", StakeStateFailed, nil},
		{"active to failed", StakeStateActive, "Fail", StakeStateFailed, nil},
		{"unstaking to failed", StakeStateUnstaking, "Fail", StakeStateFailed, nil},

		// Invalid transitions
		{"pending to active", StakeStatePending, "Activate", StakeStatePending, ErrInvalidStateTransition},
		{"pending to unstaking", StakeStatePending, "Unstake", StakeStatePending, ErrInvalidStateTransition},
		{"pending to withdrawn", StakeStatePending, "Withdraw", StakeStatePending, ErrInvalidStateTransition},
		{"active to delegating", StakeStateActive, "Delegate", StakeStateActive, ErrInvalidStateTransition},
		{"withdrawn to anything", StakeStateWithdrawn, "Delegate", StakeStateWithdrawn, ErrInvalidStateTransition},
		{"failed to anything", StakeStateFailed, "Delegate", StakeStateFailed, ErrInvalidStateTransition},
		{"withdrawn cannot fail", StakeStateWithdrawn, "Fail", StakeStateWithdrawn, ErrInvalidStateTransition},
		{"failed cannot fail", StakeStateFailed, "Fail", StakeStateFailed, ErrInvalidStateTransition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stake := &Stake{State: tt.fromState}

			var err error
			switch tt.transition {
			case "Delegate":
				err = stake.Delegate("0xvalidator", "provider-ref-1")
			case "Activate":
				err = stake.Activate()
			case "Unstake":
				err = stake.Unstake()
			case "Withdraw":
				err = stake.Withdraw()
			case "Fail":
				err = stake.Fail("test failure reason")
			}

			if tt.expectErr != nil {
				assert.ErrorIs(t, err, tt.expectErr)
				assert.Equal(t, tt.fromState, stake.State, "state should not change on invalid transition")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectState, stake.State)
			}
		})
	}
}

func TestStakeDelegate(t *testing.T) {
	t.Run("sets validator and provider ref", func(t *testing.T) {
		stake := &Stake{State: StakeStatePending}
		err := stake.Delegate("0xabc", "bitgo-ref-123")

		require.NoError(t, err)
		assert.Equal(t, "0xabc", stake.Validator)
		assert.Equal(t, "bitgo-ref-123", stake.ProviderRef)
	})
}

func TestStakeFail(t *testing.T) {
	t.Run("records failure reason", func(t *testing.T) {
		stake := &Stake{State: StakeStatePending}
		err := stake.Fail("provider timeout")

		require.NoError(t, err)
		assert.Equal(t, StakeStateFailed, stake.State)
		assert.Equal(t, "provider timeout", stake.FailureReason)
	})
}

func TestStakeIsTerminal(t *testing.T) {
	tests := []struct {
		state    StakeState
		terminal bool
	}{
		{StakeStatePending, false},
		{StakeStateDelegating, false},
		{StakeStateActive, false},
		{StakeStateUnstaking, false},
		{StakeStateWithdrawn, true},
		{StakeStateFailed, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			stake := &Stake{State: tt.state}
			assert.Equal(t, tt.terminal, stake.IsTerminal())
		})
	}
}
