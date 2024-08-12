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

	"github.com/go-logr/logr"
	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
	venapi "github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/jetstack/venafi-connection-lib/venafi_client"
	"github.com/jetstack/venafi-connection-lib/venafi_client/auth"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
)

type VenConnClient struct {
	agentMetadata *api.AgentMetadata
	connHandler   venafi_client.ConnectionHandler
	installNS     string       // Namespace in which the agent is running in.
	venConnName   string       // Name of the VenafiConnection resource to use.
	venConnNS     string       // Namespace of the VenafiConnection resource to use.
	client        *http.Client // Used to make HTTP requests to Venafi Cloud.
}

// NewVenConnClient lets you make requests to the Venafi Cloud backend using the
// given VenafiConnection resource.
//
// You need to call Start to start watching the VenafiConnection resource. If
// you don't, the client will be unable to find the VenafiConnection that you
// are referring to as its client-go cache will remain empty.
//
// The http.Client is used for Venafi and Vault, not for Kubernetes. The
// `installNS` is the namespace in which the agent is running in. The passed
// `restcfg` is not mutated. `trustedCAs` is only used for connecting to Venafi
// Cloud and Vault and can be left nil.
func NewVenConnClient(restcfg *rest.Config, agentMetadata *api.AgentMetadata, installNS, venConnName, venConnNS string, trustedCAs *x509.CertPool) (*VenConnClient, error) {
	// TODO(mael): The rest of the codebase uses the standard "log" package,
	// venafi-connection-lib uses "go-logr/logr", and client-go uses "klog". We
	// should standardize on one of them, probably "slog".
	ctrlruntimelog.SetLogger(logr.Logger{})

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
		"venafi-kubernetes-agent/"+version.PreflightVersion,
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
	if trustedCAs != nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig.RootCAs = trustedCAs
		vcpClient.Transport = tr
	}

	return &VenConnClient{
		agentMetadata: agentMetadata,
		connHandler:   handler,
		installNS:     installNS,
		venConnName:   venConnName,
		venConnNS:     venConnNS,
		client:        vcpClient,
	}, nil
}

// Start starts watching VenafiConnections. This function will return soon after
// the context is closed, or if an error occurs.
func (c *VenConnClient) Start(ctx context.Context) error {
	return c.connHandler.CacheRunnable().Start(ctx)
}

// `opts.ClusterName` and `opts.ClusterDescription` are the only values used
// from the Options struct. OrgID and ClusterID are not used in Venafi Cloud.
func (c *VenConnClient) PostDataReadingsWithOptions(readings []*api.DataReading, opts Options) error {
	if opts.ClusterName == "" {
		return fmt.Errorf("programmer mistake: the cluster name (aka `cluster_id` in the config file) cannot be left empty")
	}

	_, token, err := c.connHandler.Get(context.Background(), c.installNS, auth.Scope{}, types.NamespacedName{Name: c.venConnName, Namespace: c.venConnNS})
	if err != nil {
		return fmt.Errorf("while loading the VenafiConnection %s/%s: %w", c.venConnNS, c.venConnName, err)
	}
	if token.TPPAccessToken != "" {
		return fmt.Errorf(`VenafiConnection %s/%s: the agent cannot be used with TPP`, c.venConnNS, c.venConnName)
	}
	if token.VCPAPIKey != "" {
		// Although it is technically possible to use an API key, we have
		// decided to not allow it as it isn't recommended and will eventually
		// be phased out.
		return fmt.Errorf(`VenafiConnection %s/%s: the agent cannot be used with an API key`, c.venConnNS, c.venConnName)
	}
	if token.VCPAccessToken == "" {
		return fmt.Errorf(`programmer mistake: VenafiConnection %s/%s: TPPAccessToken is empty in the token returned by connHandler.Get: %v`, c.venConnNS, c.venConnName, token)
	}

	payload := api.DataReadingsPost{
		AgentMetadata:  c.agentMetadata,
		DataGatherTime: time.Now().UTC(),
		DataReadings:   readings,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// The path parameter "no" is a dummy parameter to make the Venafi Cloud
	// backend happy. This parameter, named `uploaderID` in the backend, is not
	// actually used by the backend.
	req, err := http.NewRequest(http.MethodPost, fullURL(token.BaseURL, "/v1/tlspk/upload/clusterdata/no"), bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("venafi-kubernetes-agent/%s", version.PreflightVersion))

	if token.VCPAccessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.VCPAccessToken))
	}
	if token.VCPAPIKey != "" {
		req.Header.Set("tppl-api-key", token.VCPAPIKey)
	}

	q := req.URL.Query()
	q.Set("name", opts.ClusterName)
	if opts.ClusterDescription != "" {
		q.Set("description", base64.RawURLEncoding.EncodeToString([]byte(opts.ClusterDescription)))
	}
	req.URL.RawQuery = q.Encode()

	res, err := c.client.Do(req)
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
func (c *VenConnClient) PostDataReadings(_orgID, _clusterID string, readings []*api.DataReading) error {
	return fmt.Errorf("programmer mistake: PostDataReadings is not implemented for Venafi Cloud")
}

// Post isn't implemented for Venafi Cloud because /v1/tlspk/upload/clusterdata
// requires using the query parameters `name` and `description` which can't be
// set using Post. Use PostDataReadingsWithOptions instead.
func (c *VenConnClient) Post(path string, body io.Reader) (*http.Response, error) {
	return nil, fmt.Errorf("programmer mistake: Post is not implemented for Venafi Cloud")
}
