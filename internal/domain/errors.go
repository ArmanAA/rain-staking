package domain

import "errors"

var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrInsufficientBalance    = errors.New("insufficient available balance")
	ErrInvalidAmount          = errors.New("amount must be positive")
	ErrStakeNotFound          = errors.New("stake not found")
	ErrBalanceNotFound        = errors.New("balance not found")
	ErrRewardNotFound         = errors.New("reward not found")
	ErrDuplicateIdempotency   = errors.New("duplicate idempotency key")
	ErrOptimisticLock         = errors.New("optimistic lock conflict: resource was modified by another request")
)
