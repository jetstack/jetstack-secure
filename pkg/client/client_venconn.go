package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	ll "sigs.k8s.io/controller-runtime/pkg/log"
)

type VenConnClient struct {
	baseURL       string // E.g., "https://api.venafi.cloud" (trailing slash will be removed)
	agentMetadata *api.AgentMetadata
	connHandler   venafi_client.ConnectionHandler
	installNS     string       // Namespace in which the agent is running in.
	venConnName   string       // Name of the VenafiConnection resource to use.
	venConnNS     string       // Namespace of the VenafiConnection resource to use.
	client        *http.Client // Used to make HTTP requests to Venafi Cloud.
}

// NewVenConnClient lets you make requests to the Venafi Cloud backend using the
// given VenafiConnection resource. You need to call Start to start watching the
// VenafiConnection resource. If you don't, the client will be unable to find
// the VenafiConnection that you are referring to as its client-go cache will
// remain empty.
func NewVenConnClient(c *http.Client, agentMetadata *api.AgentMetadata, baseURL, installNS, venConnName, venConnNS string) (*VenConnClient, error) {
	if installNS == "" {
		return nil, errors.New("programmer mistake: installNS must be provided")
	}
	if venConnName == "" {
		return nil, errors.New("programmer mistake: venConnName must be provided")
	}
	if venConnNS == "" {
		return nil, errors.New("programmer mistake: venConnNS must be provided")
	}

	cfg, err := loadRESTConfig("")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:venafi-connection", installNS),
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, &http.Client{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ll.SetLogger(logr.FromSlogHandler(slog.Default().Handler()))

	// This Kubernetes client only needs to be able to read and write the
	// VenafiConnection resources and read Secret resources.
	scheme := runtime.NewScheme()
	_ = venapi.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	handler, err := venafi_client.NewConnectionHandler(
		"venafi-kubernetes-agent/"+version.PreflightVersion,
		"venafi-kubernetes-agent.jetstack.io",
		"VenafiKubernetesAgent",
		cfg,
		scheme,
		restMapper,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &VenConnClient{
		baseURL:       baseURL,
		agentMetadata: agentMetadata,
		connHandler:   handler,
		installNS:     installNS,
		venConnName:   venConnName,
		venConnNS:     venConnNS,
		client:        c,
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
	if token.VCPAPIKey == "" && token.TPPAccessToken == "" {
		return fmt.Errorf(`programmer mistake: VenafiConnection %s/%s: no VCP API key or VCP access token was returned by connHandler.Get`, c.venConnNS, c.venConnName)
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
	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, "/v1/tlspk/upload/clusterdata/no"), bytes.NewBuffer(data))
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

// Post performs an HTTP POST request.
func (c *VenConnClient) Post(path string, body io.Reader) (*http.Response, error) {
	_, token, err := c.connHandler.Get(context.Background(), c.installNS, auth.Scope{}, types.NamespacedName{Name: c.venConnName, Namespace: c.venConnNS})
	if err != nil {
		return nil, fmt.Errorf("while loading the VenafiConnection %s/%s: %w", c.venConnNS, c.venConnName, err)
	}

	req, err := http.NewRequest(http.MethodPost, fullURL(c.baseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if len(token.BearerToken) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.BearerToken))
	}

	return c.client.Do(req)
}

func loadRESTConfig(path string) (*rest.Config, error) {
	switch path {
	// If the kubeconfig path is not provided, use the default loading rules
	// so we read the regular KUBECONFIG variable or create a non-interactive
	// client for agents running in cluster
	case "":
		loadingrules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingrules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cfg, nil
	// Otherwise use the explicitly named kubeconfig file.
	default:
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: path},
			&clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return cfg, nil
	}
}