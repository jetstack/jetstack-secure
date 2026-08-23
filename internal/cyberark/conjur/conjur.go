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

// defaultTokenTTL is the fallback cache lifetime used when a token's own
// `exp` claim can't be read — Conjur access tokens default to an 8-minute
// lifetime. Copied into a Client field on construction so tests can shrink it.
const defaultTokenTTL = 8 * time.Minute

// refreshSkew re-exchanges this long before the cached token's real expiry.
// Without it a token is served right up to the expiry instant, so a request
// that is authenticated at exp-ε but arrives at the resource server after exp
// is rejected with a 401 even though nothing is misconfigured. The skew is
// well under the 8-minute token lifetime, so it costs no extra exchanges in
// the steady state.
const refreshSkew = 30 * time.Second

// Client exchanges a JWT for a Conjur access token and authenticates requests with it.
type Client struct {
	httpClient *http.Client
	baseURL    string
	serviceID  string
	account    string
	src        jwtsource.Source
	tokenTTL   time.Duration

	mu       sync.Mutex
	token    string
	identity string
	expiry   time.Time
}

func New(httpClient *http.Client, baseURL, serviceID, account string, src jwtsource.Source) *Client {
	return &Client{httpClient: httpClient, baseURL: baseURL, serviceID: serviceID, account: account, src: src, tokenTTL: defaultTokenTTL}
}

// Invalidate clears the cached token, forcing the next AuthenticateRequest
// call to exchange a fresh one. Callers should call this after a 401 from
// the resource server the token was used against — the cache's own expiry
// tracking only catches a token aging out, not one rejected early (e.g. a
// Conjur restart or a toggled authenticator).
func (c *Client) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token, c.identity, c.expiry = "", "", time.Time{}
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
		// Conjur returns a JSON error body with the actual reason; include a
		// bounded prefix so the operator doesn't have to go read Conjur's own
		// audit log to find out why. 401 here most often means the SA token
		// audience != authenticator audience=conjur.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		// Drain the rest so the connection can be reused, bounded so a
		// misbehaving server can't make this read unboundedly.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024*1024))
		return "", fmt.Errorf("authn-jwt exchange rejected (%d): %s; verify service_id, the authenticator is enabled, and the SA token audience is 'conjur'",
			resp.StatusCode, strings.TrimSpace(string(errBody)))
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

// tokenClaims is the subset of a Conjur access token's payload this client
// reads: `sub` (the caller's identity, used for audit tagging) and `exp`
// (unix seconds, used to drive the token cache off its real expiry instead
// of a guessed TTL).
type tokenClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

// claimsFromToken extracts the payload claims from a Conjur access token.
// The payload segment is url-safe base64 without padding. Returns
// (zero value, false) if the token doesn't parse.
func claimsFromToken(token string) (tokenClaims, bool) {
	obj, ok := conjurTokenObject(token)
	if !ok {
		return tokenClaims{}, false
	}
	payloadJSON, err := base64.URLEncoding.DecodeString(padBase64(obj.Payload))
	if err != nil {
		return tokenClaims{}, false
	}
	var claims tokenClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return tokenClaims{}, false
	}
	return claims, true
}

// AuthenticateRequest implements identity.RequestAuthenticator.
//
// It exchanges the JWT for a Conjur access token, sets the Authorization
// header, and returns an identity string for audit tagging. The identity is
// the token's own `sub` claim when it can be extracted; otherwise it falls
// back to the configured service ID so a token in an unexpected shape never
// fails the request.
//
// A cached token is re-exchanged refreshSkew before its real expiry, so a
// request is never authenticated with a token that expires while it is in
// flight.
//
// The mutex is held across the exchange's network round-trip so concurrent
// callers share one exchange instead of a thundering herd; they're
// effectively serial at the current call sites. Whichever caller wins the
// race also controls the exchange's deadline via its own req.Context(), so
// an unrelated cancellation can fail a waiting caller — acceptable for now
// given the current call pattern.
func (c *Client) AuthenticateRequest(req *http.Request) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" || !time.Now().Add(refreshSkew).Before(c.expiry) {
		tok, err := c.exchange(req.Context())
		if err != nil {
			return "", err
		}
		claims, ok := claimsFromToken(tok)
		identity, expiry := c.serviceID, time.Now().Add(c.tokenTTL)
		if !ok {
			klog.FromContext(req.Context()).V(2).Info("could not parse Conjur access token; falling back to service ID as identity and a guessed expiry")
		} else {
			if claims.Sub != "" {
				identity = claims.Sub
			}
			if claims.Exp > 0 {
				expiry = time.Unix(claims.Exp, 0)
			}
		}
		c.token, c.identity, c.expiry = tok, identity, expiry
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.identity, nil
}
