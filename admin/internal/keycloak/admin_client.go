// Package keycloak contains the narrow Keycloak Admin REST API surface needed
// by the Admin service. It deliberately does not expose the service token.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"admin/internal/config"
)

var (
	ErrDisabled = errors.New("Keycloak identity management is disabled")
	ErrConflict = errors.New("Keycloak user already exists")
)

type IdentityManager interface {
	CreateAdmin(ctx context.Context, username, email, password, realmRole string) (string, error)
	DeleteUser(ctx context.Context, subject string) error
}

type AdminClient struct {
	enabled      bool
	baseURL      string
	realm        string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

var _ IdentityManager = (*AdminClient)(nil)

func NewAdminClient(cfg config.IdentityManagementConfig) (*AdminClient, error) {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil || timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &AdminClient{
		enabled: cfg.Enabled, baseURL: strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		realm: strings.TrimSpace(cfg.Realm), clientID: strings.TrimSpace(cfg.ClientID),
		clientSecret: cfg.ClientSecret, httpClient: &http.Client{Timeout: timeout},
	}, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}
type roleRepresentation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Composite   bool   `json:"composite"`
	ClientRole  bool   `json:"clientRole"`
	ContainerID string `json:"containerId"`
}

func (c *AdminClient) CreateAdmin(ctx context.Context, username, email, password, realmRole string) (string, error) {
	if !c.enabled {
		return "", ErrDisabled
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"username": username, "email": email, "enabled": true, "emailVerified": false,
		"credentials": []map[string]any{{"type": "password", "value": password, "temporary": false}},
	}
	response, err := c.doJSON(ctx, http.MethodPost, c.adminURL("users"), token, payload)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return "", ErrConflict
	}
	if response.StatusCode != http.StatusCreated {
		return "", responseError("create Keycloak user", response)
	}
	subject := pathTail(response.Header.Get("Location"))
	if subject == "" {
		return "", errors.New("Keycloak create user response has no subject")
	}
	if err := c.assignRealmRole(ctx, token, subject, realmRole); err != nil {
		_ = c.deleteWithToken(ctx, token, subject)
		return "", err
	}
	return subject, nil
}

func (c *AdminClient) DeleteUser(ctx context.Context, subject string) error {
	if !c.enabled {
		return ErrDisabled
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	return c.deleteWithToken(ctx, token, subject)
}

func (c *AdminClient) assignRealmRole(ctx context.Context, token, subject, roleName string) error {
	response, err := c.doJSON(ctx, http.MethodGet, c.adminURL("roles", url.PathEscape(roleName)), token, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return responseError("get Keycloak realm role", response)
	}
	var role roleRepresentation
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&role); err != nil {
		return fmt.Errorf("decode Keycloak realm role: %w", err)
	}
	_ = response.Body.Close()
	response, err = c.doJSON(ctx, http.MethodPost, c.adminURL("users", url.PathEscape(subject), "role-mappings", "realm"), token, []roleRepresentation{role})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return responseError("assign Keycloak realm role", response)
	}
	return nil
}

func (c *AdminClient) accessToken(ctx context.Context) (string, error) {
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {c.clientID}, "client_secret": {c.clientSecret}}
	endpoint := c.baseURL + "/realms/" + url.PathEscape(c.realm) + "/protocol/openid-connect/token"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request Keycloak service token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", responseError("request Keycloak service token", response)
	}
	var token tokenResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token); err != nil {
		return "", fmt.Errorf("decode Keycloak service token: %w", err)
	}
	if token.AccessToken == "" {
		return "", errors.New("Keycloak service token is empty")
	}
	return token.AccessToken, nil
}

func (c *AdminClient) deleteWithToken(ctx context.Context, token, subject string) error {
	response, err := c.doJSON(ctx, http.MethodDelete, c.adminURL("users", url.PathEscape(subject)), token, nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotFound {
		return responseError("delete Keycloak user", response)
	}
	return nil
}

func (c *AdminClient) doJSON(ctx context.Context, method, endpoint, token string, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Keycloak Admin API: %w", err)
	}
	return response, nil
}

func (c *AdminClient) adminURL(parts ...string) string {
	return c.baseURL + "/admin/realms/" + url.PathEscape(c.realm) + "/" + strings.Join(parts, "/")
}

func responseError(action string, response *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", action, response.StatusCode, message)
}

func pathTail(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
