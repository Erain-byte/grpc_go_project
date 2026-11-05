// Package wallet 实现钱包签名挑战和EVM签名验证。
package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"web3/internal/config"
	redisstore "web3/internal/redis"
	"web3/pkg/apperror"

	"github.com/ethereum/go-ethereum/common"
)

const nonceKeyPrefix = "wallet:nonce:"

// BindingChallenge 是返回给客户端的钱包签名挑战；NonceID 用于后续一次性消费。
type BindingChallenge struct {
	NonceID   string
	Message   string
	ExpiresAt time.Time
}

// BindingNonce 是 Redis 中保存的挑战上下文，绑定了用户、链、地址、文本和过期时间。
type BindingNonce struct {
	UserSubject string    `json:"user_subject"`
	ChainID     uint64    `json:"chain_id"`
	Address     string    `json:"address"`
	Nonce       string    `json:"nonce"`
	Message     string    `json:"message"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// NonceStore 使用 Redis 保存短期的一次性 nonce，GetDel 保证同一挑战不能重放。
type NonceStore struct {
	redis      redisstore.Client
	domain     string
	uri        string
	statement  string
	ttl        time.Duration
	nonceBytes int
	now        func() time.Time
}

// NewNonceStore 创建 nonce 存储并校验 TTL 配置。
func NewNonceStore(redis redisstore.Client, cfg config.WalletSignatureConfig) (*NonceStore, error) {
	if redis == nil {
		return nil, apperror.InvalidArgument("Redis client is nil")
	}
	ttl, err := time.ParseDuration(cfg.NonceTTL)
	if err != nil || ttl <= 0 {
		return nil, apperror.InvalidArgument("wallet nonce TTL is invalid")
	}
	return &NonceStore{
		redis:      redis,
		domain:     strings.TrimSpace(cfg.Domain),
		uri:        strings.TrimSpace(cfg.URI),
		statement:  strings.TrimSpace(cfg.Statement),
		ttl:        ttl,
		nonceBytes: cfg.NonceBytes,
		now:        time.Now,
	}, nil
}

// Create 生成加密随机 nonce，并把完整挑战上下文以 TTL 写入 Redis。
func (s *NonceStore) Create(ctx context.Context, userSubject string, chainID uint64, address string) (*BindingChallenge, error) {
	userSubject = strings.TrimSpace(userSubject)
	if userSubject == "" || chainID == 0 {
		return nil, apperror.InvalidArgument("wallet binding identity or chain ID is invalid")
	}
	normalizedAddress, err := NormalizeAddress(address)
	if err != nil {
		return nil, err
	}

	issuedAt := s.now().UTC().Truncate(time.Second)
	expiresAt := issuedAt.Add(s.ttl)
	for attempts := 0; attempts < 3; attempts++ {
		nonceID, err := randomHex(16)
		if err != nil {
			return nil, err
		}
		nonce, err := randomHex(s.nonceBytes)
		if err != nil {
			return nil, err
		}
		message := s.buildMessage(normalizedAddress, chainID, nonce, issuedAt, expiresAt)
		record := BindingNonce{
			UserSubject: userSubject,
			ChainID:     chainID,
			Address:     normalizedAddress,
			Nonce:       nonce,
			Message:     message,
			IssuedAt:    issuedAt,
			ExpiresAt:   expiresAt,
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return nil, apperror.Wrap(err, apperror.CodeInternal, "failed to encode wallet nonce", http.StatusInternalServerError)
		}
		created, err := s.redis.SetNX(ctx, nonceKeyPrefix+nonceID, payload, s.ttl)
		if err != nil {
			return nil, err
		}
		if created {
			return &BindingChallenge{NonceID: nonceID, Message: message, ExpiresAt: expiresAt}, nil
		}
	}
	return nil, apperror.Unavailable("failed to allocate a unique wallet nonce")
}

// Consume 原子读取并删除Nonce。无论后续签名是否正确，同一Nonce都不能再次使用。
func (s *NonceStore) Consume(ctx context.Context, nonceID, userSubject string) (*BindingNonce, error) {
	nonceID = strings.TrimSpace(nonceID)
	userSubject = strings.TrimSpace(userSubject)
	if nonceID == "" || userSubject == "" {
		return nil, apperror.InvalidArgument("wallet nonce identity is incomplete")
	}
	payload, err := s.redis.GetDel(ctx, nonceKeyPrefix+nonceID)
	if err != nil {
		return nil, err
	}
	if payload == "" {
		return nil, apperror.InvalidArgument("wallet nonce is missing, expired, or already used")
	}
	var record BindingNonce
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return nil, apperror.Wrap(err, apperror.CodeInternal, "wallet nonce data is invalid", http.StatusInternalServerError)
	}
	if record.UserSubject != userSubject {
		return nil, apperror.Forbidden("wallet nonce does not belong to the current user")
	}
	now := s.now().UTC()
	//判断时间是否过期
	if now.Compare(record.ExpiresAt) >= 0 {
		//if !s.now().UTC().Before(record.ExpiresAt) {
		return nil, apperror.InvalidArgument("wallet nonce has expired")
	}
	return &record, nil
}

// buildMessage 生成接近 SIWE 格式的 EIP-191 签名文本，将地址、链、nonce 和有效期绑定在一起。
func (s *NonceStore) buildMessage(address string, chainID uint64, nonce string, issuedAt, expiresAt time.Time) string {
	return fmt.Sprintf(
		"%s wants you to sign in with your Ethereum account:\n%s\n\n%s\n\nURI: %s\nVersion: 1\nChain ID: %d\nNonce: %s\nIssued At: %s\nExpiration Time: %s",
		s.domain,
		address,
		s.statement,
		s.uri,
		chainID,
		nonce,
		issuedAt.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	)
}

// randomHex 使用 crypto/rand 生成不可预测的十六进制随机值，不能用 math/rand 替代。
func randomHex(size int) (string, error) {
	if size <= 0 {
		return "", apperror.InvalidArgument("random byte size must be positive")
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", apperror.Wrap(err, apperror.CodeInternal, "failed to generate secure random value", http.StatusInternalServerError)
	}
	return hex.EncodeToString(buffer), nil
}

// NormalizeAddress 验证 EVM 地址并输出 EIP-55 校验和格式，保证数据库与缓存键的一致性。
func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if !common.IsHexAddress(address) {
		return "", apperror.InvalidArgument("invalid EVM wallet address")
	}
	return common.HexToAddress(address).Hex(), nil
}
