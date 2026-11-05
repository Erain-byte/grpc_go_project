package tron

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

func (c *Client) Allowance(ctx context.Context, owner, spender string) (*big.Int, error) {
	ownerParameter, err := ABIAddressParameter(owner)
	if err != nil {
		return nil, fmt.Errorf("encode allowance owner: %w", err)
	}
	spenderParameter, err := ABIAddressParameter(spender)
	if err != nil {
		return nil, fmt.Errorf("encode allowance spender: %w", err)
	}
	body := map[string]any{
		"contract_address":  c.usdtContract,
		"function_selector": "allowance(address,address)",
		"parameter":         ownerParameter + spenderParameter,
		"owner_address":     owner,
		"visible":           true,
	}
	return c.constantUint256(ctx, body)
}

// USDTDecimals reads decimals() from the configured official USDT contract.
func (c *Client) USDTDecimals(ctx context.Context, owner string) (uint32, error) {
	body := map[string]any{
		"contract_address":  c.usdtContract,
		"function_selector": "decimals()",
		"owner_address":     owner,
		"visible":           true,
	}
	value, err := c.constantUint256(ctx, body)
	if err != nil {
		return 0, err
	}
	if !value.IsUint64() || value.Uint64() > 255 {
		return 0, fmt.Errorf("TRON USDT decimals is outside uint8 range")
	}
	return uint32(value.Uint64()), nil
}

func (c *Client) constantUint256(ctx context.Context, body map[string]any) (*big.Int, error) {
	endpoint := resolve(c.tronGridBase, "/wallet/triggerconstantcontract")
	var response struct {
		Result struct {
			Result  bool   `json:"result"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"result"`
		ConstantResult []string `json:"constant_result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, endpoint.String(), body, &response); err != nil {
		return nil, err
	}
	if !response.Result.Result || len(response.ConstantResult) == 0 {
		return nil, fmt.Errorf("TRON constant contract call failed: %s %s", response.Result.Code, response.Result.Message)
	}
	value := new(big.Int)
	if _, ok := value.SetString(strings.TrimPrefix(response.ConstantResult[0], "0x"), 16); !ok {
		return nil, fmt.Errorf("TRON constant contract response is not valid hexadecimal")
	}
	return value, nil
}
