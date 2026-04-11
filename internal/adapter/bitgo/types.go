package bitgo

// Request/response types matching BitGo's Staking API v1.

type createStakeRequest struct {
	Amount   string `json:"amount"`
	Type     string `json:"type"`
	ClientID string `json:"clientId,omitempty"`
}

type stakingRequestResponse struct {
	ID          string       `json:"id"`
	ClientID    string       `json:"clientId"`
	Status      string       `json:"status"`
	Amount      string       `json:"amount"`
	Delegations []delegation `json:"delegations"`
}

type delegation struct {
	ID                string `json:"id"`
	DelegationAddress string `json:"delegationAddress"`
	Delegated         string `json:"delegated"`
	Rewards           string `json:"rewards"`
	Status            string `json:"status"`
}

type stakingWalletResponse struct {
	Delegated    string `json:"delegated"`
	Rewards      string `json:"rewards"`
	APY          string `json:"apy"`
	Delegations  []delegation `json:"delegations"`
}

type unstakeRequest struct {
	Amount string `json:"amount"`
	Type   string `json:"type"`
}

// BitGo status values mapped to our provider status.
const (
	bitgoStatusNew             = "NEW"
	bitgoStatusPendingApproval = "PENDING_APPROVAL"
	bitgoStatusApproved        = "APPROVED"
	bitgoStatusSigned          = "SIGNED"
	bitgoStatusDelivered       = "DELIVERED"
	bitgoStatusFailed          = "FAILED"
	bitgoStatusRejected        = "REJECTED"
)

const (
	bitgoDelegationPending = "PENDING"
	bitgoDelegationActive  = "ACTIVE"
)
