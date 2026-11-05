package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"web3/internal/auth"
	"web3/internal/config"
	"web3/internal/model"
	"web3/internal/wallet"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

// 接口实现结构体
var _ nonceManager = (*wallet.NonceStore)(nil)

// walletRepository 定义钱包业务所需的持久化能力，具体实现位于 repository 包。
type walletRepository interface {
	CreateBoundWallet(context.Context, *model.Wallet) error
	FindByID(context.Context, string, string) (*model.Wallet, error)
	FindByChainAddress(context.Context, uint64, string) (*model.Wallet, error)
	ListActiveByUser(context.Context, string) ([]model.Wallet, error)
	ListActiveByChain(context.Context, uint64) ([]model.Wallet, error)
	Reactivate(context.Context, *model.Wallet, string, time.Time) error
	UpdateLabel(context.Context, string, string, string) error
	SetDefault(context.Context, string, string) error
	UnsetDefault(context.Context, string, string) error
	Disable(context.Context, string, string) error
}

// nonceManager 创建并一次性消费钱包签名挑战，防止签名重放。
type nonceManager interface {
	Create(context.Context, string, uint64, string) (*wallet.BindingChallenge, error)
	Consume(context.Context, string, string) (*wallet.BindingNonce, error)
}

// WalletService 管理钱包绑定、解绑、默认钱包和显示标签。
// 它只保存公钥地址，绝不保存用户私钥或助记词。
type WalletService struct {
	repository walletRepository
	nonces     nonceManager
	//签名
	verifier wallet.SignatureVerifier
	//多链网关
	chains map[uint64]config.BlockchainChainConfig
	now    func() time.Time
}

// NewWalletService 仅加载启用的链，禁用链不会接受新的钱包操作。
func NewWalletService(repository walletRepository, nonces nonceManager, verifier wallet.SignatureVerifier, cfg config.BlockchainConfig) (*WalletService, error) {
	if repository == nil || nonces == nil || verifier == nil {
		return nil, apperror.InvalidArgument("wallet service dependencies are incomplete")
	}
	chains := make(map[uint64]config.BlockchainChainConfig)
	for _, chain := range cfg.Chains {
		if chain.Enabled {
			chains[chain.ChainID] = chain
		}
	}
	return &WalletService{repository: repository, nonces: nonces, verifier: verifier, chains: chains, now: time.Now}, nil
}

// GetSupportedChains 返回当前服务可用的链配置，按 chain_id 稳定排序。
func (s *WalletService) GetSupportedChains(context.Context, *web3v1.GetSupportedChainsRequest) (*web3v1.GetSupportedChainsResponse, error) {
	response := &web3v1.GetSupportedChainsResponse{Chains: make([]*web3v1.ChainInfo, 0, len(s.chains))}
	chainIDs := make([]uint64, 0, len(s.chains))
	for chainID := range s.chains {
		chainIDs = append(chainIDs, chainID)
	}
	sort.Slice(chainIDs, func(i, j int) bool { return chainIDs[i] < chainIDs[j] })
	for _, chainID := range chainIDs {
		chain := s.chains[chainID]
		response.Chains = append(response.Chains, &web3v1.ChainInfo{
			ChainId:               chain.ChainID,
			Name:                  chain.Name,
			NativeSymbol:          chain.NativeSymbol,
			NativeDecimals:        uint32(chain.NativeDecimals),
			Enabled:               true,
			RequiredConfirmations: chain.RequiredConfirmations,
		})
	}
	return response, nil
}

// CreateBindingNonce 生成一次性签名文本；客户端必须让钱包签名该文本后才能完成绑定。
func (s *WalletService) CreateBindingNonce(ctx context.Context, req *web3v1.CreateBindingNonceRequest) (*web3v1.CreateBindingNonceResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !s.chainEnabled(req.GetChainId()) {
		return nil, apperror.InvalidArgument("unsupported or disabled blockchain")
	}
	challenge, err := s.nonces.Create(ctx, identity.Subject, req.GetChainId(), req.GetAddress())
	if err != nil {
		return nil, err
	}
	return &web3v1.CreateBindingNonceResponse{
		NonceId:        challenge.NonceID,
		SigningMessage: challenge.Message,
		ExpiresAt:      timestamppb.New(challenge.ExpiresAt),
	}, nil
}

// BindWallet 消费 nonce、验证签名归属，并以链地址全局唯一约束保存绑定关系。
func (s *WalletService) BindWallet(ctx context.Context, req *web3v1.BindWalletRequest) (*web3v1.BindWalletResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || !s.chainEnabled(req.GetChainId()) {
		return nil, apperror.InvalidArgument("unsupported or disabled blockchain")
	}
	if len(req.GetLabel()) > 128 {
		return nil, apperror.InvalidArgument("wallet label is too long")
	}
	address, err := wallet.NormalizeAddress(req.GetAddress())
	if err != nil {
		return nil, err
	}
	//原子删除判断用户、链、地址、文本和过期时间否过期
	record, err := s.nonces.Consume(ctx, req.GetNonceId(), identity.Subject)
	if err != nil {
		return nil, err
	}
	if record.ChainID != req.GetChainId() || !strings.EqualFold(record.Address, address) {
		return nil, apperror.InvalidArgument("wallet binding request does not match the signed challenge")
	}
	if err := s.verifier.Verify(record.Message, req.GetSignature(), address); err != nil {
		return nil, err
	}

	existing, findErr := s.repository.FindByChainAddress(ctx, req.GetChainId(), address)
	if findErr == nil {
		if existing.UserSubject != identity.Subject {
			return nil, apperror.Conflict("wallet is already bound to another user")
		}
		if existing.Status == model.WalletStatusDisabled {
			if err := s.repository.Reactivate(ctx, existing, req.GetLabel(), s.now().UTC()); err != nil {
				return nil, err
			}
			existing, err = s.repository.FindByID(ctx, identity.Subject, existing.ID)
			if err != nil {
				return nil, err
			}
		}
		return &web3v1.BindWalletResponse{Wallet: walletToProto(existing), NewlyBound: false}, nil
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, findErr
	}

	now := s.now().UTC()
	bound := &model.Wallet{
		ID:          uuid.NewString(),
		UserSubject: identity.Subject,
		ChainID:     req.GetChainId(),
		Address:     address,
		Label:       req.GetLabel(),
		Status:      model.WalletStatusActive,
		VerifiedAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repository.CreateBoundWallet(ctx, bound); err != nil {
		// 并发绑定同一地址时返回数据库中已经胜出的记录。
		winner, lookupErr := s.repository.FindByChainAddress(ctx, req.GetChainId(), address)
		if lookupErr == nil && winner.UserSubject == identity.Subject {
			return &web3v1.BindWalletResponse{Wallet: walletToProto(winner), NewlyBound: false}, nil
		}
		if lookupErr == nil {
			return nil, apperror.Conflict("wallet is already bound to another user")
		}
		return nil, err
	}
	return &web3v1.BindWalletResponse{Wallet: walletToProto(bound), NewlyBound: true}, nil
}

// UnbindWallet 逻辑删除当前用户的钱包；历史交易记录不会被删除。
func (s *WalletService) UnbindWallet(ctx context.Context, req *web3v1.UnbindWalletRequest) (*web3v1.UnbindWalletResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetWalletId()) == "" {
		return nil, apperror.InvalidArgument("wallet ID is empty")
	}
	if err := s.repository.Disable(ctx, identity.Subject, req.GetWalletId()); err != nil {
		return nil, normalizeNotFound(err)
	}
	return &web3v1.UnbindWalletResponse{}, nil
}

// GetWallets 只返回当前用户仍启用的钱包。
func (s *WalletService) GetWallets(ctx context.Context, _ *web3v1.GetWalletsRequest) (*web3v1.GetWalletsResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	wallets, err := s.repository.ListActiveByUser(ctx, identity.Subject)
	if err != nil {
		return nil, err
	}
	response := &web3v1.GetWalletsResponse{Wallets: make([]*web3v1.WalletInfo, 0, len(wallets))}
	for i := range wallets {
		response.Wallets = append(response.Wallets, walletToProto(&wallets[i]))
	}
	return response, nil
}

// UpdateWallet 允许修改标签或默认钱包状态，不允许在此修改链和地址。
func (s *WalletService) UpdateWallet(ctx context.Context, req *web3v1.UpdateWalletRequest) (*web3v1.UpdateWalletResponse, error) {
	identity, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetWalletId()) == "" {
		return nil, apperror.InvalidArgument("wallet ID is empty")
	}
	if req.Label != nil {
		if len(req.GetLabel()) > 128 {
			return nil, apperror.InvalidArgument("wallet label is too long")
		}
		if err := s.repository.UpdateLabel(ctx, identity.Subject, req.GetWalletId(), req.GetLabel()); err != nil {
			return nil, normalizeNotFound(err)
		}
	}
	if req.IsDefault != nil {
		if req.GetIsDefault() {
			err = s.repository.SetDefault(ctx, identity.Subject, req.GetWalletId())
		} else {
			err = s.repository.UnsetDefault(ctx, identity.Subject, req.GetWalletId())
		}
		if err != nil {
			return nil, normalizeNotFound(err)
		}
	}
	updated, err := s.repository.FindByID(ctx, identity.Subject, req.GetWalletId())
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	return &web3v1.UpdateWalletResponse{Wallet: walletToProto(updated)}, nil
}

func (s *WalletService) chainEnabled(chainID uint64) bool {
	_, ok := s.chains[chainID]
	return ok
}

// requireIdentity 从认证中间件写入的 Context 读取调用方身份。
func requireIdentity(ctx context.Context) (auth.Identity, error) {
	identity, ok := auth.RequireIdentity(ctx)
	if !ok {
		return auth.Identity{}, apperror.Unauthorized("authenticated user identity is missing")
	}
	return identity, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperror.NotFound("wallet was not found")
	}
	return err
}

func walletToProto(value *model.Wallet) *web3v1.WalletInfo {
	if value == nil {
		return nil
	}
	return &web3v1.WalletInfo{
		Id:         value.ID,
		ChainId:    value.ChainID,
		Address:    value.Address,
		Label:      value.Label,
		Status:     web3v1.WalletStatus(value.Status),
		VerifiedAt: timestamppb.New(value.VerifiedAt),
		CreatedAt:  timestamppb.New(value.CreatedAt),
		UpdatedAt:  timestamppb.New(value.UpdatedAt),
		IsDefault:  value.IsDefault,
	}
}
