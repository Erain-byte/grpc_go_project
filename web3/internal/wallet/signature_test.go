package wallet

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestEIP191VerifierAcceptsWalletSignature(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	message := "bind wallet challenge"
	signature, err := crypto.Sign(eip191Hash(message), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟personal_sign常见的27/28恢复标识。
	signature[64] += 27
	encoded := "0x" + hex.EncodeToString(signature)
	expected := crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	if err := NewEIP191Verifier().Verify(message, encoded, expected); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
}

func TestEIP191VerifierRejectsDifferentWallet(t *testing.T) {
	privateKey, _ := crypto.GenerateKey()
	otherKey, _ := crypto.GenerateKey()
	message := "bind wallet challenge"
	signature, _ := crypto.Sign(eip191Hash(message), privateKey)
	encoded := hex.EncodeToString(signature)
	otherAddress := crypto.PubkeyToAddress(otherKey.PublicKey).Hex()

	if err := NewEIP191Verifier().Verify(message, encoded, otherAddress); err == nil {
		t.Fatal("expected signature mismatch")
	}
}
