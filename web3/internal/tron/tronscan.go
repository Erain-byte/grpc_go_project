package tron

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

type ApprovalRecord struct {
	Amount          string    `json:"amount"`
	Unlimited       bool      `json:"unlimited"`
	FromAddress     string    `json:"from_address"`
	ToAddress       string    `json:"to_address"`
	ContractAddress string    `json:"contract_address"`
	TokenInfo       TokenInfo `json:"tokenInfo"`
	Project         Project   `json:"project"`
	OperateTime     int64     `json:"operate_time"`
}

type TokenInfo struct {
	TokenID      string `json:"tokenId"`
	TokenDecimal int    `json:"tokenDecimal"`
	TokenLevel   string `json:"tokenLevel"`
}

type Project struct {
	Name string `json:"name"`
}

type AddressInfo struct {
	Tag1          string          `json:"tag1"`
	Name          string          `json:"name"`
	PublicTag     string          `json:"publicTag"`
	PublicTagDesc string          `json:"publicTagDesc"`
	Risk          json.RawMessage `json:"risk"`
}

type ApprovalPage struct {
	Total             int                    `json:"total"`
	RangeTotal        int                    `json:"rangeTotal"`
	Data              []ApprovalRecord       `json:"data"`
	ContractMap       map[string]bool        `json:"contractMap"`
	ContractInfo      map[string]AddressInfo `json:"contractInfo"`
	NormalAddressInfo map[string]AddressInfo `json:"normalAddressInfo"`
}

type approvalRiskResponse struct {
	RiskApprove []ApprovalRecord `json:"riskApprove"`
}

func (c *Client) ListApprovals(ctx context.Context, address string, start, limit int) (*ApprovalPage, error) {
	endpoint := resolve(c.tronScanBase, "/api/account/approve/list")
	query := endpoint.Query()
	query.Set("address", address)
	query.Set("start", strconv.Itoa(start))
	query.Set("limit", strconv.Itoa(limit))
	query.Set("type", "token")
	endpoint.RawQuery = query.Encode()

	var response ApprovalPage
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, &response); err != nil {
		return nil, err
	}
	if response.Data == nil {
		response.Data = []ApprovalRecord{}
	}
	return &response, nil
}

func (c *Client) RiskySpenders(ctx context.Context, address string) (map[string]struct{}, error) {
	endpoint := resolve(c.tronScanBase, "/api/security/auth/data")
	query := endpoint.Query()
	query.Set("address", address)
	endpoint.RawQuery = query.Encode()

	var response approvalRiskResponse
	if err := c.doJSON(ctx, http.MethodGet, endpoint.String(), nil, &response); err != nil {
		return nil, err
	}
	result := make(map[string]struct{})
	for _, approval := range response.RiskApprove {
		if approval.ContractAddress == c.usdtContract {
			result[approval.ToAddress] = struct{}{}
		}
	}
	return result, nil
}
