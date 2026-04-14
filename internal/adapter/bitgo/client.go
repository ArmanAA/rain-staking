package bitgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/ArmanAA/rain-staking/internal/port"
)

// Client implements port.StakingProvider using BitGo's Staking API v1.
type Client struct {
	baseURL     string
	accessToken string
	walletID    string
	coin        string
	httpClient  *http.Client
	logger      *slog.Logger
}

func NewClient(baseURL, accessToken, walletID, coin string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:     baseURL,
		accessToken: accessToken,
		walletID:    walletID,
		coin:        coin,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) Stake(ctx context.Context, req port.StakeRequest) (*port.StakeResponse, error) {
	body := createStakeRequest{
		Amount:   toWei(req.Amount),
		Type:     "STAKE",
		ClientID: req.ClientRef,
	}

	var resp stakingRequestResponse
	err := c.doRequest(ctx, http.MethodPost, c.stakingPath("/requests"), body, &resp)
	if err != nil {
		return nil, fmt.Errorf("bitgo stake request: %w", err)
	}

	validator := ""
	if len(resp.Delegations) > 0 {
		validator = resp.Delegations[0].DelegationAddress
	}

	return &port.StakeResponse{
		ProviderRef: resp.ID,
		Validator:   validator,
	}, nil
}

func (c *Client) Unstake(ctx context.Context, providerRef string) error {
	body := unstakeRequest{
		Type: "UNSTAKE",
	}

	var resp stakingRequestResponse
	path := c.stakingPath(fmt.Sprintf("/requests/%s/unstake", providerRef))
	err := c.doRequest(ctx, http.MethodPost, path, body, &resp)
	if err != nil {
		return fmt.Errorf("bitgo unstake request: %w", err)
	}
	return nil
}

func (c *Client) GetStakeStatus(ctx context.Context, providerRef string) (*port.StakeStatusResponse, error) {
	var resp stakingWalletResponse
	err := c.doRequest(ctx, http.MethodGet, c.stakingPath(""), nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("bitgo get stake status: %w", err)
	}

	status := port.ProviderStakeStatusPending
	validator := ""

	for _, d := range resp.Delegations {
		if d.Status == bitgoDelegationActive {
			status = port.ProviderStakeStatusActive
			validator = d.DelegationAddress
			break
		}
	}

	return &port.StakeStatusResponse{
		Status:    status,
		Validator: validator,
	}, nil
}

func (c *Client) GetRewards(ctx context.Context, providerRef string) ([]port.RewardEntry, error) {
	var resp stakingWalletResponse
	err := c.doRequest(ctx, http.MethodGet, c.stakingPath(""), nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("bitgo get rewards: %w", err)
	}

	var entries []port.RewardEntry
	for _, d := range resp.Delegations {
		if d.Rewards != "" && d.Rewards != "0" {
			amount, err := fromWei(d.Rewards)
			if err != nil {
				c.logger.WarnContext(ctx, "failed to parse reward amount",
					slog.String("raw", d.Rewards), slog.String("error", err.Error()))
				continue
			}
			entries = append(entries, port.RewardEntry{
				Amount:     amount,
				RewardDate: time.Now().UTC().Format("2006-01-02"),
			})
		}
	}

	return entries, nil
}

func (c *Client) stakingPath(suffix string) string {
	return fmt.Sprintf("%s/api/staking/v1/%s/wallets/%s%s", c.baseURL, c.coin, c.walletID, suffix)
}

func (c *Client) doRequest(ctx context.Context, method, url string, body any, result any) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	c.logger.DebugContext(ctx, "bitgo request", slog.String("method", method), slog.String("url", url))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("bitgo API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// toWei converts ETH decimal to Wei string (1 ETH = 10^18 Wei).
func toWei(amount decimal.Decimal) string {
	wei := amount.Mul(decimal.New(1, 18))
	return wei.StringFixed(0)
}

// fromWei converts Wei string to ETH decimal.
func fromWei(weiStr string) (decimal.Decimal, error) {
	wei, err := decimal.NewFromString(weiStr)
	if err != nil {
		return decimal.Zero, err
	}
	return wei.Div(decimal.New(1, 18)), nil
}
