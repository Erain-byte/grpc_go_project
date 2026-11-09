package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"admin/internal/config"
)

func TestAdminClientCreateAdmin(t *testing.T) {
	t.Parallel()
	var created, roleAssigned bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/realms/grpc-project/protocol/openid-connect/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "admin-management" {
				t.Fatalf("unexpected token request: %v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "service-token"})
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/grpc-project/users":
			created = true
			w.Header().Set("Location", "http://keycloak/admin/realms/grpc-project/users/subject-1")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/grpc-project/roles/admin":
			_ = json.NewEncoder(w).Encode(roleRepresentation{ID: "role-1", Name: "admin"})
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/grpc-project/users/subject-1/role-mappings/realm":
			roleAssigned = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAdminClient(config.IdentityManagementConfig{
		Enabled: true, BaseURL: server.URL, Realm: "grpc-project", ClientID: "admin-management", ClientSecret: "secret", Timeout: "2s",
	})
	if err != nil {
		t.Fatal(err)
	}
	subject, err := client.CreateAdmin(context.Background(), "alice", "alice@example.com", "password-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if subject != "subject-1" || !created || !roleAssigned {
		t.Fatalf("unexpected result: subject=%q created=%v assigned=%v", subject, created, roleAssigned)
	}
}

func TestAdminClientCreateConflict(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/grpc-project/protocol/openid-connect/token" {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "service-token"})
			return
		}
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()
	client, _ := NewAdminClient(config.IdentityManagementConfig{Enabled: true, BaseURL: server.URL, Realm: "grpc-project", ClientID: "admin-management", ClientSecret: "secret", Timeout: "2s"})
	_, err := client.CreateAdmin(context.Background(), "alice", "alice@example.com", "password-1", "admin")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestDisabledAdminClient(t *testing.T) {
	t.Parallel()
	client, _ := NewAdminClient(config.IdentityManagementConfig{Timeout: "2s"})
	_, err := client.CreateAdmin(context.Background(), "alice", "alice@example.com", "password-1", "admin")
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected disabled, got %v", err)
	}
}
