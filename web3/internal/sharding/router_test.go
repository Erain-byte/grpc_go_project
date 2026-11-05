package sharding

import "testing"

// 路由结果一旦改变，旧钱包的历史交易会被查询到错误的分表。
func TestTransactionRouterKeepsStableMapping(t *testing.T) {
	router := NewTransactionRouter()
	tests := []struct {
		walletID string
		table    string
	}{
		{"00000000-0000-0000-0000-000000000001", "wallet_transactions_06"},
		{"550e8400-e29b-41d4-a716-446655440000", "wallet_transactions_00"},
	}

	for _, test := range tests {
		table, err := router.TableName(test.walletID)
		if err != nil {
			t.Fatalf("route %s: %v", test.walletID, err)
		}
		if table != test.table {
			t.Fatalf("route %s: got %s, want %s", test.walletID, table, test.table)
		}
	}
}
