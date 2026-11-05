package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"web3/internal/auth"
	"web3/internal/config"
	"web3/internal/model"
	"web3/internal/wallet"
	"web3/pkg/apperror"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"gorm.io/gorm"
)

const (
	testSubject = "018f64a8-2551-7b50-a94d-0f4fbc80a13b"
	testAddress = "0x000000000000000000000000000000000000dEaD"
)

type fakeWalletRepository struct {
	byAddress      *model.Wallet
	byChainAddress map[string]*model.Wallet
	created        *model.Wallet
}

func (r *fakeWalletRepository) CreateBoundWallet(_ context.Context, value *model.Wallet) error {
	r.created = value
	r.byAddress = value
	return nil
}
func (r *fakeWalletRepository) FindByID(_ context.Context, _, _ string) (*model.Wallet, error) {
	if r.byAddress == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.byAddress, nil
}
func (r *fakeWalletRepository) FindByChainAddress(_ context.Context, chainID uint64, address string) (*model.Wallet, error) {
	if r.byChainAddress != nil {
		wallet := r.byChainAddress[fmt.Sprintf("%d:%s", chainID, strings.ToLower(address))]
		if wallet == nil {
			return nil, gorm.ErrRecordNotFound
		}
		return wallet, nil
	}
	if r.byAddress == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return r.byAddress, nil
}
func (*fakeWalletRepository) ListActiveByUser(context.Context, string) ([]model.Wallet, error) {
	return nil, nil
}
func (r *fakeWalletRepository) ListActiveByChain(_ context.Context, chainID uint64) ([]model.Wallet, error) {
	if r.byChainAddress == nil {
		if r.byAddress == nil || r.byAddress.ChainID != chainID {
			return nil, nil
		}
		return []model.Wallet{*r.byAddress}, nil
	}
	seen := make(map[string]struct{}, len(r.byChainAddress))
	result := make([]model.Wallet, 0, len(r.byChainAddress))
	for _, wallet := range r.byChainAddress {
		if wallet.ChainID != chainID {
			continue
		}
		if _, ok := seen[wallet.ID]; ok {
			continue
		}
		seen[wallet.ID] = struct{}{}
		result = append(result, *wallet)
	}
	return result, nil
}
func (*fakeWalletRepository) Reactivate(context.Context, *model.Wallet, string, time.Time) error {
	return nil
}
func (*fakeWalletRepository) UpdateLabel(context.Context, string, string, string) error { return nil }
func (*fakeWalletRepository) SetDefault(context.Context, string, string) error          { return nil }
func (*fakeWalletRepository) UnsetDefault(context.Context, string, string) error        { return nil }
func (*fakeWalletRepository) Disable(context.Context, string, string) error             { return nil }

type fakeNonceManager struct {
	record *wallet.BindingNonce
}

func (*fakeNonceManager) Create(context.Context, string, uint64, string) (*wallet.BindingChallenge, error) {
	return nil, errors.New("not used")
}
func (m *fakeNonceManager) Consume(context.Context, string, string) (*wallet.BindingNonce, error) {
	return m.record, nil
}

type fakeSignatureVerifier struct {
	called bool
}

func (*fakeSignatureVerifier) RecoverAddress(string, string) (string, error) {
	return "", errors.New("not used")
}
func (v *fakeSignatureVerifier) Verify(_, _, _ string) error {
	v.called = true
	return nil
}

func TestBindWalletCreatesVerifiedBinding(t *testing.T) {
	repository := &fakeWalletRepository{}
	nonces := &fakeNonceManager{record: &wallet.BindingNonce{
		UserSubject: testSubject,
		ChainID:     1,
		Address:     testAddress,
		Message:     "sign this challenge",
	}}
	verifier := &fakeSignatureVerifier{}
	service, err := NewWalletService(repository, nonces, verifier, enabledChains(1))
	if err != nil {
		t.Fatalf("NewWalletService() error = %v", err)
	}

	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	response, err := service.BindWallet(ctx, &web3v1.BindWalletRequest{
		ChainId:   1,
		Address:   testAddress,
		NonceId:   "nonce-1",
		Signature: "signature",
		Label:     "primary",
	})
	if err != nil {
		t.Fatalf("BindWallet() error = %v", err)
	}
	if !verifier.called || repository.created == nil {
		t.Fatal("BindWallet() did not verify and persist the wallet")
	}
	if !response.GetNewlyBound() || response.GetWallet().GetAddress() != testAddress {
		t.Fatalf("BindWallet() response = %+v", response)
	}
}

func TestBindWalletRejectsAddressOwnedByAnotherUser(t *testing.T) {
	repository := &fakeWalletRepository{byAddress: &model.Wallet{
		ID:          "wallet-1",
		UserSubject: "018f64a8-2551-7b50-a94d-0f4fbc80ffff",
		ChainID:     1,
		Address:     testAddress,
		Status:      model.WalletStatusActive,
	}}
	service, err := NewWalletService(repository, &fakeNonceManager{record: &wallet.BindingNonce{
		UserSubject: testSubject,
		ChainID:     1,
		Address:     testAddress,
		Message:     "sign this challenge",
	}}, &fakeSignatureVerifier{}, enabledChains(1))
	if err != nil {
		t.Fatalf("NewWalletService() error = %v", err)
	}

	ctx := auth.NewContext(context.Background(), auth.Identity{Subject: testSubject})
	_, err = service.BindWallet(ctx, &web3v1.BindWalletRequest{
		ChainId: 1, Address: testAddress, NonceId: "nonce-1", Signature: "signature",
	})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConflict {
		t.Fatalf("BindWallet() error = %v, want conflict", err)
	}
}

func TestGetSupportedChainsUsesStableOrder(t *testing.T) {
	service, err := NewWalletService(&fakeWalletRepository{}, &fakeNonceManager{}, &fakeSignatureVerifier{}, enabledChains(137, 1))
	if err != nil {
		t.Fatalf("NewWalletService() error = %v", err)
	}
	response, err := service.GetSupportedChains(context.Background(), &web3v1.GetSupportedChainsRequest{})
	if err != nil {
		t.Fatalf("GetSupportedChains() error = %v", err)
	}
	if len(response.GetChains()) != 2 || response.GetChains()[0].GetChainId() != 1 || response.GetChains()[1].GetChainId() != 137 {
		t.Fatalf("GetSupportedChains() chain order = %+v", response.GetChains())
	}
}

func enabledChains(chainIDs ...uint64) config.BlockchainConfig {
	chains := make([]config.BlockchainChainConfig, 0, len(chainIDs))
	for _, chainID := range chainIDs {
		chains = append(chains, config.BlockchainChainConfig{ChainID: chainID, Name: "test", Enabled: true})
	}
	return config.BlockchainConfig{Chains: chains}
}
