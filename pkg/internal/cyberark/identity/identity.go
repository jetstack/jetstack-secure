package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/logs"
	"github.com/jetstack/preflight/pkg/version"
)

const (
	// MechanismUsernamePassword is the string which identifies the username/password mechanism for completing
	// a login attempt
	MechanismUsernamePassword = "UP"

	// ActionAnswer is the string which is sent to an AdvanceAuthentication request to indicate we're providing
	// the credentials in band in text format (i.e., we're sending a password)
	ActionAnswer = "Answer"

	// SummaryLoginSuccess is returned by a StartAuthentication to indicate that login does not need
	// to proceed to the AdvanceAuthentication step.
	// We don't handle this because we don't expect it to happen.
	SummaryLoginSuccess = "LoginSuccess"

	// SummaryNewPackage is returned by a StartAuthentication call when the user must complete a challenge
	// to complete the log in. This is expected on a first login.
	SummaryNewPackage = "NewPackage"

	// maxStartAuthenticationBodySize is the maximum allowed size for a response body from the CyberArk Identity
	// StartAuthentication endpoint.
	// As of 2025-04-30, a response from the integration environment is ~1kB
	maxStartAuthenticationBodySize = 10 * 1024

	// maxAdvanceAuthenticationBodySize is the maximum allowed size for a response body from the CyberArk Identity
	// AdvanceAuthentication endpoint.
	// As of 2025-04-30, a response from the integration environment is ~3kB
	maxAdvanceAuthenticationBodySize = 30 * 1024
)

var (
	errNoUPMechanism = fmt.Errorf("found no authentication mechanism with the username + password type (%s); unable to complete login using this identity", MechanismUsernamePassword)
)

// startAuthenticationRequestBody is the body sent to the StartAuthentication endpoint in CyberArk Identity;
// see https://api-docs.cyberark.com/docs/identity-api-reference/authentication-and-authorization/operations/create-a-security-start-authentication
type startAuthenticationRequestBody struct {
	// TenantID is the internal ID of the tenant containing the user attempting to log in. In testing,
	// it seems that the subdomain works in this field.
	TenantID string `json:"TenantId"`

	// Version is set to 1.0
	Version string `json:"Version"`

	// User is the username of the user trying to log in. For a human, this is likely to be an email address.
	User string `json:"User"`
}

// identityResponseBody generically wraps a response from the Identity server; the Result will differ for
// responses from different endpoint, but the other fields are similar.
// Not all fields in the JSON returned from the server are replicated here, since we only need a subset.
type identityResponseBody[T any] struct {
	// Success is a simple boolean indicator from the server of success.
	// NB: The JSON key is lowercase, in contrast to other JSON keys in the response.
	Success bool `json:"success"`

	// Result holds the information we need to parse from successful responses
	Result T `json:"Result"`

	// Message holds an information message such as an error message. Experimentally it seems to be null
	// for successful attempts.
	Message string `json:"Message"`

	// ErrorID holds an error ID when something goes wrong with the call.
	// Not to be confused with ErrorCode; for failure messages, we see ErrorID set and ErrorCode null.
	ErrorID string `json:"ErrorID"`

	// NB: Other fields omitted since we don't need them
}

// startAuthenticationResponseBody is the response returned by the server from a request to StartAuthentication.
type startAuthenticationResponseBody identityResponseBody[startAuthenticationResponseResult]

// advanceAuthenticationResponseBody is the response from the AdvanceAuthentication endpoint.
type advanceAuthenticationResponseBody identityResponseBody[advanceAuthenticationResponseResult]

// startAuthenticationResponseResult holds the important data we need to pass to AdvanceAuthentication
type startAuthenticationResponseResult struct {
	// SessionID identifies this login attempt, and must be passed with the
	// follow-up AdvanceAuthentication request.
	SessionID string `json:"SessionId"`

	// Challenges provides a list of methods for logging in. We need to look
	// for the correct login method we want to use, and then find the MechanismId
	// for that login method to pass to the AdvanceAuthentication request.
	Challenges []startAuthenticationChallenge `json:"Challenges"`

	// Summary indicates whether a StartAuthentication calls needs to be followed up with an AdvanceAuthentication
	// call. From the docs:
	// > If the user exists, the response contains a Summary of either LoginSuccess or NewPackage.
	// > You receive LoginSuccess when the request includes an .ASPXAUTH cookie from prior successful authentication.
	Summary string `json:"Summary"`
}

// startAuthenticationChallenge is an entry in the array of MFA mechanisms;
// at least one MFA mechanism should be satisfied by the user.
type startAuthenticationChallenge struct {
	Mechanisms []startAuthenticationMechanism `json:"Mechanisms"`
}

// startAuthenticationMechanism holds details of a given mechanism for authenticating.
// This corresponds to "how" the user authenticates, e.g. via password or email, etc
type startAuthenticationMechanism struct {
	// Name represents the name of the challenge mechanism. This is usually an upper-case
	// string, such as "UP" for "username / password"
	Name string `json:"Name"`

	// Enrolled is true if the given mechanism is available for the user attempting
	// to authenticate.
	Enrolled bool `json:"Enrolled"`

	// MechanismID uniquely identifies a particular mechanism, and must be passed
	// to the AdvanceAuthentication request when authenticating.
	MechanismID string `json:"MechanismId"`
}

// advanceAuthenticationRequestBody is a request body for the AdvanceAuthentication call to CyberArk Identity,
// which should usually be obtained by making requests to StartAuthentication first.
// WARNING: This struct can hold secret data (a user's password)
type advanceAuthenticationRequestBody struct {
	// Action is a string identifying how we're intending to log in; for username/password, this is
	// set to "Answer" to indicate that the password is held in the Answer field
	Action string `json:"Action"`

	// Answer holds the user's password to send to the server
	// WARNING: THIS IS SECRET DATA.
	Answer string `json:"Answer"`

	// MechanismID identifies the login mechanism and must be retrieved from a call to StartAuthentication
	MechanismID string `json:"MechanismId"`

	// SessionID identifies the login session and must be retrieved from a call to StartAuthentication
	SessionID string `json:"SessionId"`

	// TenantID identifies the tenant; this can be inferred from the URL if we used service discovery to
	// get the Identity API URL, but we set it anyway to be explicit.
	TenantID string `json:"TenantId"`

	// PersistentLogin is documented to "[indicate] whether the session should persist after the user
	// closes the browser"; for service-to-service auth which we're trying to do, we set this to true.
	PersistentLogin bool `json:"PersistentLogin"`
}

// advanceAuthenticationResponseResult is the specific information returned for a successful AdvanceAuthentication call
type advanceAuthenticationResponseResult struct {
	// Summary holds a "brief summary of the authentication outcome"
	Summary string `json:"Summary"`

	// Token is the auth token we need to save; this is the result of the login
	// process which can be sent as a bearer token to other services.
	Token string `json:"Token"`

	// Other fields omitted as they're not needed
}

// Client is an client for interacting with the CyberArk Identity API and performing a login using a username and password.
// For context on the behaviour of this client, see the Python SDK: https://github.com/cyberark/ark-sdk-python/blob/3be12c3f2d3a2d0407025028943e584b6edc5996/ark_sdk_python/auth/identity/ark_identity.py
type Client struct {
	client *http.Client

	endpoint  string
	subdomain string

	tokenCached      token
	tokenCachedMutex sync.Mutex
}

// token is a wrapper type for holding auth tokens we want to cache.
type token string

// New returns an initialized CyberArk Identity client using a default service discovery client.
// NB: This function performs service discovery when called, in order to ensure that all Identity
// clients are created with a valid Identity API URL. This function blocks on the network call to
// the discovery service.
func New(ctx context.Context, subdomain string) (*Client, error) {
	return NewWithDiscoveryClient(ctx, servicediscovery.New(), subdomain)
}

// NewWithDiscoveryClient returns an initialized CyberArk Identity client using the given service discovery client.
// NB: This function performs service discovery when called, in order to ensure that all Identity
// clients are created with a valid Identity API URL. This function blocks on the network call to
// the discovery service.
func NewWithDiscoveryClient(ctx context.Context, discoveryClient *servicediscovery.Client, subdomain string) (*Client, error) {
	if discoveryClient == nil {
		return nil, fmt.Errorf("must provide a non-nil discovery client to the Identity Client")
	}

	endpoint, err := discoveryClient.DiscoverIdentityAPIURL(ctx, subdomain)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport.NewDebuggingRoundTripper(http.DefaultTransport, transport.DebugByContext),
		},

		endpoint:  endpoint,
		subdomain: subdomain,

		tokenCached:      "",
		tokenCachedMutex: sync.Mutex{},
	}, nil
}

// LoginUsernamePassword performs a blocking call to fetch an auth token from CyberArk Identity using the given username and password.
// The password is zeroed after use.
// Tokens are cached internally and are not directly accessible to code; use Client.AuthenticateRequest to add credentials
// to an *http.Request.
func (c *Client) LoginUsernamePassword(ctx context.Context, username string, password []byte) error {
	defer func() {
		for i := range password {
			password[i] = 0x00
		}
	}()

	operation := func() (any, error) {
		advanceRequestBody, err := c.doStartAuthentication(ctx, username)
		if err != nil {
			return struct{}{}, err
		}

		// NB: We explicitly pass advanceRequestBody by value here so that when we add the password
		// in doAdvanceAuthentication we don't create a copy of the password slice elsewhere.
		err = c.doAdvanceAuthentication(ctx, username, &password, advanceRequestBody)
		if err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	}

	backoffPolicy := backoff.NewConstantBackOff(10 * time.Second)

	_, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoffPolicy))

	return err
}

// doStartAuthentication performs the initial request to start the login process using a username and password.
// It returns a partially initialized advanceAuthenticationRequestBody ready to send to the server to complete
// the login. As this function doesn't have access to the password, it must be added to the returned request body
// by the caller before being used as a request to AdvanceAuthentication.
// See https://api-docs.cyberark.com/docs/identity-api-reference/authentication-and-authorization/operations/create-a-security-start-authentication
func (c *Client) doStartAuthentication(ctx context.Context, username string) (advanceAuthenticationRequestBody, error) {
	response := advanceAuthenticationRequestBody{}

	logger := klog.FromContext(ctx).WithValues("source", "Identity.doStartAuthentication")

	body := startAuthenticationRequestBody{
		Version: "1.0", // this is the only value in the docs

		TenantID: c.subdomain,

		User: username,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return response, fmt.Errorf("failed to marshal JSON for request to StartAuthentication endpoint: %s", err)
	}

	endpoint, err := url.JoinPath(c.endpoint, "Security", "StartAuthentication")
	if err != nil {
		return response, fmt.Errorf("failed to create URL for request to CyberArk Identity StartAuthentication: %s", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return response, fmt.Errorf("failed to initialise request to Identity endpoint %s: %s", endpoint, err)
	}

	setIdentityHeaders(request)

	httpResponse, err := c.client.Do(request)
	if err != nil {
		return response, fmt.Errorf("failed to perform HTTP request to start authentication: %s", err)
	}

	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		err := fmt.Errorf("got unexpected status code %s from request to start authentication in CyberArk Identity API", httpResponse.Status)
		if httpResponse.StatusCode >= 500 || httpResponse.StatusCode < 400 {
			return response, err
		}

		// If we got a 4xx error, we shouldn't retry
		return response, backoff.Permanent(err)

	}

	startAuthResponse := startAuthenticationResponseBody{}

	err = json.NewDecoder(io.LimitReader(httpResponse.Body, maxStartAuthenticationBodySize)).Decode(&startAuthResponse)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return response, fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}

		return response, fmt.Errorf("failed to parse JSON from otherwise successful request to start authentication: %s", err)
	}

	if !startAuthResponse.Success {
		return response, fmt.Errorf("got a failure response from request to start authentication: message=%q, error=%q", startAuthResponse.Message, startAuthResponse.ErrorID)
	}

	logger.V(logs.Debug).Info("made successful request to StartAuthentication", "summary", startAuthResponse.Result.Summary)

	if startAuthResponse.Result.Summary != SummaryNewPackage {
		// This means we can't respond to whatever summary the server sent.
		// The best thing to do is try and find a challenge we can solve anyway.
		klog.FromContext(ctx).Info("got an unexpected Summary from StartAuthentication response; will attempt to complete a login challenge anyway", "summary", startAuthResponse.Result.Summary)
	}

	// We can only handle a UP type challenge, and if there are any other challenges, we'll have to fail because we can't handle them.
	// https://github.com/cyberark/ark-sdk-python/blob/3be12c3f2d3a2d0407025028943e584b6edc5996/ark_sdk_python/auth/identity/ark_identity.py#L405
	switch len(startAuthResponse.Result.Challenges) {
	case 0:
		return response, fmt.Errorf("got no valid challenges in response to start authentication; unable to log in")

	case 1:
		// do nothing, this is ideal

	default:
		return response, fmt.Errorf("got %d challenges in response to start authentication, which means MFA may be enabled; unable to log in", len(startAuthResponse.Result.Challenges))
	}

	challenge := startAuthResponse.Result.Challenges[0]

	switch len(challenge.Mechanisms) {
	case 0:
		// presumably this shouldn't happen, but handle the case anyway
		return response, fmt.Errorf("got no mechanisms for challenge from Identity server")

	case 1:
		// do nothing, this is ideal

	default:
		return response, fmt.Errorf("got %d mechanisms in response to start authentication, which means MFA may be enabled; unable to log in", len(challenge.Mechanisms))
	}

	mechanism := challenge.Mechanisms[0]

	if !mechanism.Enrolled || mechanism.Name != MechanismUsernamePassword {
		return response, errNoUPMechanism
	}

	response.Action = ActionAnswer
	response.MechanismID = mechanism.MechanismID
	response.SessionID = startAuthResponse.Result.SessionID
	response.TenantID = c.subdomain
	response.PersistentLogin = true

	return response, nil
}

// doAdvanceAuthentication performs the second step of the login process, sending the password to the server
// and receiving a token in response.
func (c *Client) doAdvanceAuthentication(ctx context.Context, username string, password *[]byte, requestBody advanceAuthenticationRequestBody) error {
	if password == nil {
		return backoff.Permanent(fmt.Errorf("password must not be nil; this is a programming error"))
	}

	requestBody.Answer = string(*password)

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return backoff.Permanent(fmt.Errorf("failed to marshal JSON for request to AdvanceAuthentication endpoint: %s", err))
	}

	endpoint, err := url.JoinPath(c.endpoint, "Security", "AdvanceAuthentication")
	if err != nil {
		return backoff.Permanent(fmt.Errorf("failed to create URL for request to CyberArk Identity AdvanceAuthentication: %s", err))
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to initialise request to Identity endpoint %s: %s", endpoint, err)
	}

	setIdentityHeaders(request)

	httpResponse, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to perform HTTP request to advance authentication: %s", err)
	}

	defer httpResponse.Body.Close()

	// Important: Even login failures can produce a 200 status code, so this
	// check won't catch all failures
	if httpResponse.StatusCode != http.StatusOK {
		err := fmt.Errorf("got unexpected status code %s from request to advance authentication in CyberArk Identity API", httpResponse.Status)
		if httpResponse.StatusCode >= 500 || httpResponse.StatusCode < 400 {
			return err
		}

		// If we got a 4xx error, we shouldn't retry
		return backoff.Permanent(err)
	}

	advanceAuthResponse := advanceAuthenticationResponseBody{}

	err = json.NewDecoder(io.LimitReader(httpResponse.Body, maxAdvanceAuthenticationBodySize)).Decode(&advanceAuthResponse)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			return fmt.Errorf("rejecting JSON response from server as it was too large or was truncated")
		}

		return fmt.Errorf("failed to parse JSON from otherwise successful request to advance authentication: %s", err)
	}

	if !advanceAuthResponse.Success {
		return fmt.Errorf("got a failure response from request to advance authentication: message=%q, error=%q", advanceAuthResponse.Message, advanceAuthResponse.ErrorID)
	}

	if advanceAuthResponse.Result.Summary != SummaryLoginSuccess {
		// IF MFA was enabled and we got here, there's probably nothing to be gained from a retry
		// and the best thing to do is fail now so the user can fix MFA settings.
		return backoff.Permanent(fmt.Errorf("got a %s response from AdvanceAuthentication; this implies that the user account %s requires MFA, which is not supported. Try unlocking MFA for this user", advanceAuthResponse.Result.Summary, username))
	}

	klog.FromContext(ctx).Info("successfully completed AdvanceAuthentication request to CyberArk Identity; login complete", "username", username)

	c.tokenCachedMutex.Lock()

	c.tokenCached = token(advanceAuthResponse.Result.Token)

	c.tokenCachedMutex.Unlock()

	return nil
}

// setIdentityHeaders sets the headers required for requests to the CyberArk Identity API.
// From the docs:
// Your request header must contain X-IDAP-NATIVE-CLIENT:true to indicate that an application is invoking
// the CyberArk Identity endpoint, and
// Content-Type: application/json to indicate that the body is in JSON format.
// Experimentally, it seems the X-IDAP-NATIVE-CLIENT is not required but we'll follow the docs.
func setIdentityHeaders(r *http.Request) {
	// The "canonicalheader" linter warns us that the IDAP-NATIVE-CLIENT header isn't canonical, but we silence it here
	// since we want to exactly match the docs.
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-IDAP-NATIVE-CLIENT", "true") //nolint: canonicalheader
	version.SetUserAgent(r)
}
