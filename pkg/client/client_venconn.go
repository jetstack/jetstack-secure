package client

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	venapi "github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/jetstack/venafi-connection-lib/chain/sources/venafi"
	"github.com/jetstack/venafi-connection-lib/venafi_client"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
)

type VenConnClient struct {
	agentMetadata *api.AgentMetadata
	connHandler   venafi_client.ConnectionHandler
	installNS     string // Namespace in which the agent is running in.
	venConnName   string // Name of the VenafiConnection resource to use.
	venConnNS     string // Namespace of the VenafiConnection resource to use.

	// Used to make HTTP requests to Venafi Cloud. This field is public for
	// testing purposes so that we can configure trusted CAs; there should be a
	// way to do that without messing with the client directly (e.g., a flag to
	// pass a custom CA?), but it's not there yet.
	Client *http.Client
}

// NewVenConnClient lets you make requests to the Venafi Cloud backend using the
// given VenafiConnection resource.
//
// You need to call Start to start watching the VenafiConnection resource. If
// you don't, the client will be unable to find the VenafiConnection that you
// are referring to as its client-go cache will remain empty.
//
// The http.Client is used for Venafi and Vault, not for Kubernetes. The
// `installNS` is the namespace in which the agent is running in and cannot be
// empty. `venConnName` and `venConnNS` must not be empty either. The passed
// `restcfg` is not mutated. `trustedCAs` is only used for connecting to Venafi
// Cloud and Vault and can be left nil.
func NewVenConnClient(restcfg *rest.Config, agentMetadata *api.AgentMetadata, installNS, venConnName, venConnNS string, trustedCAs *x509.CertPool) (*VenConnClient, error) {
	if installNS == "" {
		return nil, errors.New("programmer mistake: installNS must be provided")
	}
	if venConnName == "" {
		return nil, errors.New("programmer mistake: venConnName must be provided")
	}
	if venConnNS == "" {
		return nil, errors.New("programmer mistake: venConnNS must be provided")
	}

	restcfg = rest.CopyConfig(restcfg)
	restcfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:venafi-connection", installNS),
	}

	// TLS-related configuration such as root CAs and client certs are contained
	// in the restcfg; let's create an http.Client that uses them.
	httpCl, err := rest.HTTPClientFor(restcfg)
	if err != nil {
		return nil, fmt.Errorf("while turning the REST config into an HTTP client: %w", err)
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(restcfg, httpCl)
	if err != nil {
		return nil, fmt.Errorf("while creating the REST mapper: %w", err)
	}

	// This Kubernetes client only needs to be able to read and write the
	// VenafiConnection resources and read Secret resources.
	scheme := runtime.NewScheme()
	_ = venapi.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	handler, err := venafi_client.NewConnectionHandler(
		version.UserAgent(),
		"venafi-kubernetes-agent.jetstack.io",
		"VenafiKubernetesAgent",
		restcfg,
		scheme,
		restMapper,
		trustedCAs,
	)
	if err != nil {
		return nil, err
	}

	vcpClient := &http.Client{}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if trustedCAs != nil {
		tr.TLSClientConfig.RootCAs = trustedCAs
	}
	vcpClient.Transport = transport.DebugWrappers(tr)

	return &VenConnClient{
		agentMetadata: agentMetadata,
		connHandler:   handler,
		installNS:     installNS,
		venConnName:   venConnName,
		venConnNS:     venConnNS,
		Client:        vcpClient,
	}, nil
}

// Start starts watching VenafiConnections. This function will return soon after
// the context is closed, or if an error occurs.
func (c *VenConnClient) Start(ctx context.Context) error {
	return c.connHandler.CacheRunnable().Start(ctx)
}

// `opts.ClusterName` and `opts.ClusterDescription` are the only values used
// from the Options struct. OrgID and ClusterID are not used in Venafi Cloud.
func (c *VenConnClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, opts Options) error {
	if opts.ClusterName == "" {
		return fmt.Errorf("programmer mistake: the cluster name (aka `cluster_id` in the config file) cannot be left empty")
	}

	_, details, err := c.connHandler.Get(ctx, c.installNS, venafi.Scope{}, types.NamespacedName{Name: c.venConnName, Namespace: c.venConnNS})
	if err != nil {
		return fmt.Errorf("while loading the VenafiConnection %s/%s: %w", c.venConnNS, c.venConnName, err)
	}
	if details.TPP != nil {
		return fmt.Errorf(`VenafiConnection %s/%s: the agent cannot be used with TPP`, c.venConnNS, c.venConnName)
	}
	if details.VCP != nil && details.VCP.APIKey != "" {
		// Although it is technically possible to use an API key, we have
		// decided to not allow it as it isn't recommended and will eventually
		// be phased out.
		return fmt.Errorf(`VenafiConnection %s/%s: the agent cannot be used with an API key`, c.venConnNS, c.venConnName)
	}
	if details.VCP == nil || details.VCP.AccessToken == "" {
		return fmt.Errorf(`programmer mistake: VenafiConnection %s/%s: TPPAccessToken is empty in the token returned by connHandler.Get: %v`, c.venConnNS, c.venConnName, details)
	}

	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}

	encodedBody := &bytes.Buffer{}

	err = json.NewEncoder(encodedBody).Encode(payload)
	if err != nil {
		return err
	}

	// The path parameter "no" is a dummy parameter to make the Venafi Cloud
	// backend happy. This parameter, named `uploaderID` in the backend, is not
	// actually used by the backend.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL(details.VCP.URL, "/v1/tlspk/upload/clusterdata/no"), encodedBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", details.VCP.AccessToken))
	version.SetUserAgent(req)

	q := req.URL.Query()
	q.Set("name", opts.ClusterName)
	if opts.ClusterDescription != "" {
		q.Set("description", base64.RawURLEncoding.EncodeToString([]byte(opts.ClusterDescription)))
	}
	req.URL.RawQuery = q.Encode()

	res, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if code := res.StatusCode; code < 200 || code >= 300 {
		errorContent := ""
		body, err := io.ReadAll(res.Body)
		if err == nil {
			errorContent = string(body)
		}

		return fmt.Errorf("received response with status code %d. Body: [%s]", code, errorContent)
	}

	return nil
}

// PostDataReadings isn't implemented for Venafi Cloud. This is because Venafi
// Cloud needs a `clusterName` and `clusterDescription`, but this function can
// only pass `orgID` and `clusterID` which are both useless in Venafi Cloud. Use
// PostDataReadingsWithOptions instead.
func (c *VenConnClient) PostDataReadings(_ context.Context, _orgID, _clusterID string, readings []*api.DataReading) error {
	return fmt.Errorf("programmer mistake: PostDataReadings is not implemented for Venafi Cloud")
}

// Post isn't implemented for Venafi Cloud because /v1/tlspk/upload/clusterdata
// requires using the query parameters `name` and `description` which can't be
// set using Post. Use PostDataReadingsWithOptions instead.
func (c *VenConnClient) Post(_ context.Context, path string, body io.Reader) (*http.Response, error) {
	return nil, fmt.Errorf("programmer mistake: Post is not implemented for Venafi Cloud")
}
