package wallet

import (
	"encoding/hex"
	"fmt"
	"strings"

	"web3/pkg/apperror"

	"github.com/ethereum/go-ethereum/crypto"
)

// SignatureVerifier 抽象钱包签名恢复和归属校验，便于测试替换。
type SignatureVerifier interface {
	RecoverAddress(message, signature string) (string, error)
	Verify(message, signature, expectedAddress string) error
}

// EIP191Verifier 验证常见浏览器钱包的 personal_sign / EIP-191 签名。
type EIP191Verifier struct{}

var _ SignatureVerifier = (*EIP191Verifier)(nil)

// NewEIP191Verifier 创建 EIP-191 验证器。
func NewEIP191Verifier() *EIP191Verifier { return &EIP191Verifier{} }

// RecoverAddress 从签名恢复公钥地址；不会信任请求中提交的地址字段。
func (v *EIP191Verifier) RecoverAddress(message, signature string) (string, error) {
	signature = strings.TrimPrefix(strings.TrimSpace(signature), "0x")
	decoded, err := hex.DecodeString(signature)
	if err != nil || len(decoded) != crypto.SignatureLength {
		return "", apperror.InvalidArgument("wallet signature must be a 65-byte hex value")
	}

	// 浏览器钱包常返回27/28，go-ethereum恢复函数要求0/1。
	if decoded[64] == 27 || decoded[64] == 28 {
		decoded[64] -= 27
	}
	if decoded[64] > 1 {
		return "", apperror.InvalidArgument("wallet signature recovery ID is invalid")
	}

	publicKey, err := crypto.SigToPub(eip191Hash(message), decoded)
	if err != nil {
		return "", apperror.InvalidArgument("wallet signature cannot be recovered")
	}
	return crypto.PubkeyToAddress(*publicKey).Hex(), nil
}

// eip191Hash 增加 Ethereum Signed Message 前缀，必须与钱包的 personal_sign 规则一致。
func eip191Hash(message string) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	return crypto.Keccak256([]byte(prefix), []byte(message))
}

// Verify 比较签名恢复出的地址与用户请求绑定的地址，二者不一致则拒绝绑定。
func (v *EIP191Verifier) Verify(message, signature, expectedAddress string) error {
	expectedAddress, err := NormalizeAddress(expectedAddress)
	if err != nil {
		return err
	}
	recoveredAddress, err := v.RecoverAddress(message, signature)
	if err != nil {
		return err
	}
	if !strings.EqualFold(recoveredAddress, expectedAddress) {
		return apperror.Unauthorized("wallet signature does not match the requested address")
	}
	return nil
}
