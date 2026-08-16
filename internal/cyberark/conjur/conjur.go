package conjur

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/internal/cyberark/jwtsource"
)

const tokenTTL = 8 * time.Minute

// Client exchanges a JWT for a Conjur access token and authenticates requests with it.
type Client struct {
	httpClient *http.Client
	baseURL    string
	serviceID  string
	account    string
	src        jwtsource.Source

	mu        sync.Mutex
	token     string
	identity  string
	tokenTime time.Time
}

func New(httpClient *http.Client, baseURL, serviceID, account string, src jwtsource.Source) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, serviceID: serviceID, account: account, src: src}
}

func (c *Client) exchange(ctx context.Context) (string, error) {
	jwt, err := c.src.Read(ctx)
	if err != nil {
		return "", err
	}
	endpoint, err := url.JoinPath(c.baseURL, "authn-jwt", c.serviceID, c.account, "authenticate")
	if err != nil {
		return "", err
	}
	form := url.Values{"jwt": {jwt}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Request the base64-encoded access token — Conjur's canonical wire form
	// for the token, and the encoding this client's own decoding below
	// expects.
	req.Header.Set("Accept-Encoding", "base64")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("authn-jwt exchange transport error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 401 here most often means the SA token audience != authenticator audience=conjur
		return "", fmt.Errorf("authn-jwt exchange rejected (%d): verify service_id, the authenticator is enabled, and the SA token audience is 'conjur'", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

// padBase64 adds the '=' padding base64.StdEncoding/URLEncoding require,
// for inputs that arrived without it.
func padBase64(s string) string {
	return s + strings.Repeat("=", (4-len(s)%4)%4)
}

// flattenedJWSJSON is the wire shape of a Conjur access token: a Flattened
// JWS JSON Serialization object, optionally base64-encoded on top (Conjur's
// `Accept-Encoding: base64`, which this client requests).
type flattenedJWSJSON struct {
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// conjurTokenObject parses a Conjur access token into its Flattened-JWS-JSON
// object, tolerating the token being raw JSON, standard base64, or
// url-safe base64 (Conjur may return any of these depending on encoding).
func conjurTokenObject(token string) (*flattenedJWSJSON, bool) {
	candidates := []string{token}
	padded := padBase64(token)
	if decoded, err := base64.StdEncoding.DecodeString(padded); err == nil {
		candidates = append(candidates, string(decoded))
	}
	if decoded, err := base64.URLEncoding.DecodeString(padded); err == nil {
		candidates = append(candidates, string(decoded))
	}
	for _, candidate := range candidates {
		var obj flattenedJWSJSON
		if err := json.Unmarshal([]byte(candidate), &obj); err != nil {
			continue
		}
		if obj.Protected != "" && obj.Payload != "" && obj.Signature != "" {
			return &obj, true
		}
	}
	return nil, false
}

// identityFromToken extracts the `sub` claim from a Conjur access token's
// payload. The payload segment is url-safe base64 without padding. Returns
// ("", false) if the token doesn't parse or has no `sub` claim.
func identityFromToken(token string) (string, bool) {
	obj, ok := conjurTokenObject(token)
	if !ok {
		return "", false
	}
	payloadJSON, err := base64.URLEncoding.DecodeString(padBase64(obj.Payload))
	if err != nil {
		return "", false
	}
	var payload struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return "", false
	}
	if payload.Sub == "" {
		return "", false
	}
	return payload.Sub, true
}

// AuthenticateRequest implements identity.RequestAuthenticator.
// It exchanges the JWT for a Conjur access token, sets the Authorization
// header, and returns an identity string for audit tagging. The identity is
// the token's own `sub` claim when it can be extracted; otherwise it falls
// back to the configured service ID so a token in an unexpected shape never
// fails the request.
func (c *Client) AuthenticateRequest(req *http.Request) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" || time.Since(c.tokenTime) >= tokenTTL {
		tok, err := c.exchange(req.Context())
		if err != nil {
			return "", err
		}
		identity, ok := identityFromToken(tok)
		if !ok {
			klog.FromContext(req.Context()).V(2).Info("could not extract sub claim from Conjur access token; falling back to service ID as identity")
			identity = c.serviceID
		}
		c.token, c.identity, c.tokenTime = tok, identity, time.Now()
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.identity, nil
}
