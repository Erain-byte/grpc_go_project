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
