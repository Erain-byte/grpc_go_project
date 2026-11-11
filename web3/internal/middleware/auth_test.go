package middleware

import (
	"context"
	"testing"

	web3v1 "github.com/Erain-byte/grpc_go_project/proto/web3/v1"
	"google.golang.org/grpc/metadata"
)

func TestResolveWeb3AuthRule(t *testing.T) {
	rule, err := resolveAuthRule(web3v1.Web3Service_BindWallet_FullMethodName)
	if err != nil {
		t.Fatal(err)
	}
	if len(rule.GetRoles()) != 1 || rule.GetRoles()[0] != "user" {
		t.Fatalf("unexpected BindWallet roles: %v", rule.GetRoles())
	}
}

func TestResolveFeedbackListAuthRule(t *testing.T) {
	rule, err := resolveAuthRule(web3v1.Web3Service_GetFeedbackList_FullMethodName)
	if err != nil {
		t.Fatal(err)
	}
	if rule.GetPublic() {
		t.Fatal("GetFeedbackList must not be public")
	}
	if len(rule.GetRoles()) != 2 || rule.GetRoles()[0] != "admin" || rule.GetRoles()[1] != "super-admin" {
		t.Fatalf("unexpected GetFeedbackList roles: %v", rule.GetRoles())
	}
}

func TestResolveRuntimeStatusAuthRule(t *testing.T) {
	rule, err := resolveAuthRule(web3v1.Web3Service_GetWeb3RuntimeStatus_FullMethodName)
	if err != nil {
		t.Fatal(err)
	}
	if rule.GetPublic() {
		t.Fatal("GetWeb3RuntimeStatus must not be public")
	}
	if len(rule.GetRoles()) != 2 || rule.GetRoles()[0] != "admin" || rule.GetRoles()[1] != "super-admin" {
		t.Fatalf("unexpected GetWeb3RuntimeStatus roles: %v", rule.GetRoles())
	}
}

func TestResolveAirdropAuthRule(t *testing.T) {
	rule, err := resolveAuthRule(web3v1.Web3Service_CheckEvmAirdrops_FullMethodName)
	if err != nil {
		t.Fatal(err)
	}
	if !rule.GetPublic() || len(rule.GetRoles()) != 0 {
		t.Fatalf("CheckEvmAirdrops must be public: %+v", rule)
	}
}

func TestBearerTokenFromMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token-value"))
	token, err := bearerTokenFromMetadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token-value" {
		t.Fatalf("token = %q", token)
	}
}

func TestRoleAllowedNormalizesSeparator(t *testing.T) {
	if !roleAllowed("super_admin", []string{"super-admin"}) {
		t.Fatal("expected normalized role to be allowed")
	}
	if roleAllowed("user", []string{"admin"}) {
		t.Fatal("unexpected role permission")
	}
}
