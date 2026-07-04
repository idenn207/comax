// Package client is the HTTP client for the Comax Secrets server.
//
// Shared between the CLI (M1) and the future Go SDK (M5). The package
// stays in pkg/ rather than internal/ so external consumers can import
// it without a fork. Keep the surface small and stable; new endpoints
// added in later milestones should be additive.
package client

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
)

// Client wraps an *http.Client with the bearer token and base URL.
// Construct via New. Zero value is unusable on purpose: every CLI call
// needs both the server URL and the token.
type Client struct {
	base   *url.URL
	token  string
	http   *http.Client
	userAg string
}

// New returns a configured client. baseURL must include the scheme
// (http:// or https://). token is the bearer credential — pass "" only
// for endpoints that don't require auth (currently just /healthz and
// /bootstrap).
//
// timeout is a per-request deadline; pass 0 for the package default
// (10s). Cold-start budget (Task 11) is sensitive to net.Dial timeout
// so we do not block longer than necessary.
func New(baseURL, token string, timeout time.Duration) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("client: baseURL is required")
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("client: parse baseURL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("client: baseURL %q must use http or https", baseURL)
	}
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		base:   u,
		token:  token,
		http:   &http.Client{Timeout: timeout},
		userAg: "comax-secret-cli/dev",
	}, nil
}

// Envelope mirrors the server's response shape. Callers don't reference
// this directly — the typed Get* / Put* helpers below unmarshal Data
// into their natural type.
type Envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *ErrorBody      `json:"error,omitempty"`
}

// ErrorBody is the failure shape returned by every endpoint.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIError is the failure type returned to callers when the server
// rejects a request. Inspect .Code (or use errors.Is against
// ErrUnauthorized / ErrNotFound) to drive CLI exit codes.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API %d %s: %s", e.Status, e.Code, e.Message)
}

// Sentinel errors for the common API failure codes. CLI commands use
// these to print friendly messages without parsing strings.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

// Is implements errors.Is so APIError can match against the sentinels
// above based on its Code field.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrUnauthorized:
		return e.Code == "unauthorized"
	case ErrNotFound:
		return e.Code == "not_found"
	case ErrConflict:
		return e.Code == "conflict" || e.Code == "already_bootstrapped"
	}
	return false
}

// do performs a request and unmarshals Data into out. body is JSON-encoded.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	target, err := c.base.Parse(strings.TrimPrefix(path, "/"))
	if err != nil {
		return fmt.Errorf("client: build URL %q: %w", path, err)
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("client: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return fmt.Errorf("client: new request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("User-Agent", c.userAg)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("client: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("client: read body: %w", err)
	}
	var env Envelope
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("client: parse envelope (status=%d, body=%q): %w", resp.StatusCode, raw, err)
		}
	}
	if resp.StatusCode >= 400 {
		ae := &APIError{Status: resp.StatusCode}
		if env.Error != nil {
			ae.Code = env.Error.Code
			ae.Message = env.Error.Message
		} else {
			ae.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return ae
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("client: parse data: %w", err)
		}
	}
	return nil
}

// ---- High-level operations -----------------------------------------------

// Health returns nil on a healthy server, an error otherwise.
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/healthz", nil, nil)
}

// BootstrapResponse is what /bootstrap returns. token is shown once.
type BootstrapResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

// Bootstrap mints the initial admin token. Fails with ErrConflict if
// the server has already been bootstrapped.
func (c *Client) Bootstrap(ctx context.Context) (BootstrapResponse, error) {
	var out BootstrapResponse
	err := c.do(ctx, http.MethodPost, "/api/v1/bootstrap", nil, &out)
	return out, err
}

// Project is the JSON shape returned by /projects.
type Project struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ListProjects returns every project the bearer can see.
func (c *Client) ListProjects(ctx context.Context) ([]Project, error) {
	var out []Project
	err := c.do(ctx, http.MethodGet, "/api/v1/projects", nil, &out)
	return out, err
}

// CreateProject creates a project by name. Returns ErrConflict if the
// name is taken.
func (c *Client) CreateProject(ctx context.Context, name string) (Project, error) {
	var out Project
	err := c.do(ctx, http.MethodPost, "/api/v1/projects",
		map[string]string{"name": name}, &out)
	return out, err
}

// Env is the JSON shape returned by /envs.
type Env struct {
	ID           int64     `json:"id"`
	ProjectID    int64     `json:"project_id"`
	Name         string    `json:"name"`
	InheritsFrom string    `json:"inherits_from,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListEnvs returns every env under projectName.
func (c *Client) ListEnvs(ctx context.Context, projectName string) ([]Env, error) {
	var out []Env
	err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/projects/%s/envs", url.PathEscape(projectName)), nil, &out)
	return out, err
}

// CreateEnv creates an env. inheritsFrom is "" when none.
func (c *Client) CreateEnv(ctx context.Context, projectName, envName, inheritsFrom string) (Env, error) {
	var out Env
	body := map[string]string{"name": envName}
	if inheritsFrom != "" {
		body["inherits_from"] = inheritsFrom
	}
	err := c.do(ctx, http.MethodPost,
		fmt.Sprintf("/api/v1/projects/%s/envs", url.PathEscape(projectName)), body, &out)
	return out, err
}

// Secret is the resolved-plaintext shape returned by /secrets endpoints.
type Secret struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListSecrets returns every resolved secret in env.
func (c *Client) ListSecrets(ctx context.Context, projectName, envName string) ([]Secret, error) {
	var out []Secret
	err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/projects/%s/envs/%s/secrets",
			url.PathEscape(projectName), url.PathEscape(envName)), nil, &out)
	return out, err
}

// GetSecret returns one resolved secret.
func (c *Client) GetSecret(ctx context.Context, projectName, envName, key string) (Secret, error) {
	var out Secret
	err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/projects/%s/envs/%s/secrets/%s",
			url.PathEscape(projectName), url.PathEscape(envName), url.PathEscape(key)), nil, &out)
	return out, err
}

// PutSecret upserts one secret.
func (c *Client) PutSecret(ctx context.Context, projectName, envName, key, value string) (Secret, error) {
	var out Secret
	err := c.do(ctx, http.MethodPut,
		fmt.Sprintf("/api/v1/projects/%s/envs/%s/secrets/%s",
			url.PathEscape(projectName), url.PathEscape(envName), url.PathEscape(key)),
		map[string]string{"value": value}, &out)
	return out, err
}

// Token is the metadata shape returned by GET /api/v1/tokens. The token
// hash is never present — a listing carries only non-secret fields.
// LastUsedAt / RevokedAt are nil when unset.
type Token struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	IsAdmin    bool       `json:"is_admin"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// TokenCreated is returned by CreateToken. Token is the plaintext bearer,
// shown exactly once — the server persists only its SHA-256 hash.
type TokenCreated struct {
	Token     string    `json:"token"`
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

// ListTokens returns every service token (metadata only). The server
// requires an admin bearer; a non-admin caller receives an *APIError with
// Code "forbidden".
func (c *Client) ListTokens(ctx context.Context) ([]Token, error) {
	var out []Token
	err := c.do(ctx, http.MethodGet, "/api/v1/tokens", nil, &out)
	return out, err
}

// CreateToken issues a new non-admin service token named name and returns
// its plaintext (shown once). Requires an admin bearer.
func (c *Client) CreateToken(ctx context.Context, name string) (TokenCreated, error) {
	var out TokenCreated
	err := c.do(ctx, http.MethodPost, "/api/v1/tokens",
		map[string]string{"name": name}, &out)
	return out, err
}

// RevokeToken soft-revokes the token with the given id. Returns
// ErrNotFound (via *APIError) when the id is unknown or already revoked.
// Requires an admin bearer.
func (c *Client) RevokeToken(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/api/v1/tokens/%d", id), nil, nil)
}

// Webhook is the listing shape from GET /api/v1/webhooks. The signing secret
// is never present — it is shown once at creation. Env is nil for an all-envs
// subscription.
type Webhook struct {
	ID        int64     `json:"id"`
	Project   string    `json:"project"`
	Env       *string   `json:"env,omitempty"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookCreated is returned by CreateWebhook. SigningSecret is the plaintext
// HMAC key, shown exactly once — the server persists only its ciphertext.
type WebhookCreated struct {
	ID            int64     `json:"id"`
	Project       string    `json:"project"`
	Env           *string   `json:"env,omitempty"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	Enabled       bool      `json:"enabled"`
	SigningSecret string    `json:"signing_secret"`
	CreatedAt     time.Time `json:"created_at"`
}

// Delivery is one row from GET /api/v1/webhooks/{id}/deliveries.
type Delivery struct {
	ID            int64      `json:"id"`
	Event         string     `json:"event"`
	Status        string     `json:"status"`
	Attempts      int64      `json:"attempts"`
	LastStatus    *int64     `json:"last_status,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	NextAttemptAt time.Time  `json:"next_attempt_at"`
	CreatedAt     time.Time  `json:"created_at"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
}

// CreateWebhookInput is the request body for CreateWebhook. Env is optional
// (empty = all environments in the project); Events empty = all event kinds.
type CreateWebhookInput struct {
	Project string   `json:"project"`
	Env     string   `json:"env,omitempty"`
	URL     string   `json:"url"`
	Events  []string `json:"events,omitempty"`
}

// CreateWebhook registers a webhook and returns its signing secret (shown
// once). Requires an admin bearer. A malformed or SSRF-blocked URL returns an
// *APIError with Code "bad_request".
func (c *Client) CreateWebhook(ctx context.Context, in CreateWebhookInput) (WebhookCreated, error) {
	var out WebhookCreated
	err := c.do(ctx, http.MethodPost, "/api/v1/webhooks", in, &out)
	return out, err
}

// ListWebhooks returns every webhook (metadata only). Requires an admin bearer.
func (c *Client) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	var out []Webhook
	err := c.do(ctx, http.MethodGet, "/api/v1/webhooks", nil, &out)
	return out, err
}

// DeleteWebhook removes a webhook by id. Returns ErrNotFound (via *APIError)
// when the id is unknown. Requires an admin bearer.
func (c *Client) DeleteWebhook(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete,
		fmt.Sprintf("/api/v1/webhooks/%d", id), nil, nil)
}

// SetWebhookEnabled toggles a webhook's enabled flag (soft-disable). A disabled
// webhook stops receiving new deliveries but keeps its registration and
// history. Returns ErrNotFound (via *APIError) when the id is unknown. Requires
// an admin bearer.
func (c *Client) SetWebhookEnabled(ctx context.Context, id int64, enabled bool) error {
	return c.do(ctx, http.MethodPatch,
		fmt.Sprintf("/api/v1/webhooks/%d", id),
		map[string]bool{"enabled": enabled}, nil)
}

// ListDeliveries returns the recent delivery attempts for a webhook. Requires
// an admin bearer.
func (c *Client) ListDeliveries(ctx context.Context, webhookID int64) ([]Delivery, error) {
	var out []Delivery
	err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/webhooks/%d/deliveries", webhookID), nil, &out)
	return out, err
}
