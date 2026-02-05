package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/kubeconfig"
)

// OIDCDiscovery contains the configuration for the oidc data-gatherer.
type OIDCDiscovery struct {
	// KubeConfigPath is the path to the kubeconfig file. If empty, will assume it runs in-cluster.
	KubeConfigPath string `yaml:"kubeconfig"`
}

// UnmarshalYAML unmarshals the Config resolving GroupVersionResource.
func (c *OIDCDiscovery) UnmarshalYAML(unmarshal func(any) error) error {
	aux := struct {
		KubeConfigPath string `yaml:"kubeconfig"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return err
	}

	c.KubeConfigPath = aux.KubeConfigPath

	return nil
}

func (c *OIDCDiscovery) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	cl, err := kubeconfig.NewDiscoveryClient(c.KubeConfigPath)
	if err != nil {
		return nil, err
	}

	return &DataGathererOIDC{
		cl: cl.RESTClient(),
	}, nil
}

// DataGathererOIDC stores the config for an oidc datagatherer.
type DataGathererOIDC struct {
	cl rest.Interface
}

var _ datagatherer.DataGatherer = &DataGathererOIDC{}

func (g *DataGathererOIDC) Run(ctx context.Context) error {
	return nil
}

func (g *DataGathererOIDC) WaitForCacheSync(ctx context.Context) error {
	// no async functionality, see Fetch
	return nil
}

// Fetch will fetch the OIDC discovery document and JWKS from the cluster API server.
func (g *DataGathererOIDC) Fetch(ctx context.Context) (any, int, error) {
	oidcResponse, oidcErr := g.fetchOIDCConfig(ctx)
	jwksResponse, jwksErr := g.fetchJWKS(ctx)

	errToString := func(err error) string {
		if err != nil {
			return err.Error()
		}
		return ""
	}

	if oidcErr != nil {
		klog.FromContext(ctx).V(4).Error(oidcErr, "Failed to fetch OIDC configuration")
	}
	if jwksErr != nil {
		klog.FromContext(ctx).V(4).Error(jwksErr, "Failed to fetch JWKS")
	}

	return &api.OIDCDiscoveryData{
		OIDCConfig:      oidcResponse,
		OIDCConfigError: errToString(oidcErr),
		JWKS:            jwksResponse,
		JWKSError:       errToString(jwksErr),
	}, 1 /* we have 1 result, so return 1 as count */, nil
}

func (g *DataGathererOIDC) fetchOIDCConfig(ctx context.Context) (map[string]any, error) {
	// Fetch the OIDC discovery document from the well-known endpoint.
	result := g.cl.Get().AbsPath("/.well-known/openid-configuration").Do(ctx)
	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to get /.well-known/openid-configuration: %s", k8sErrorMessage(err))
	}

	bytes, _ := result.Raw() // we already checked result.Error(), so there is no error here
	var oidcResponse map[string]any
	if err := json.Unmarshal(bytes, &oidcResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OIDC discovery document: %v (raw: %q)", err, stringFirstN(string(bytes), 80))
	}

	return oidcResponse, nil
}

func (g *DataGathererOIDC) fetchJWKS(ctx context.Context) (map[string]any, error) {
	// Fetch the JWKS from the default /openid/v1/jwks endpoint.
	// We are not using the jwks_uri from the OIDC config because:
	//  - on hybrid OpenShift clusters, we saw it pointed to a non-existent URL
	//  - on fully private AWS EKS clusters, the URL is still public and might not
	//    be reachable from within the cluster (https://github.com/aws/containers-roadmap/issues/2038)
	// So we are using the default path instead, which we think should work in most cases.
	result := g.cl.Get().AbsPath("/openid/v1/jwks").Do(ctx)
	if err := result.Error(); err != nil {
		return nil, fmt.Errorf("failed to get /openid/v1/jwks: %s", k8sErrorMessage(err))
	}

	bytes, _ := result.Raw() // we already checked result.Error(), so there is no error here
	var jwksResponse map[string]any
	if err := json.Unmarshal(bytes, &jwksResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWKS response: %v (raw: %q)", err, stringFirstN(string(bytes), 80))
	}

	return jwksResponse, nil
}

func stringFirstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// based on https://github.com/kubernetes/kubectl/blob/a64ceaeab69eed1f11a9e1bd91cf2c1446de811c/pkg/cmd/util/helpers.go#L244
func k8sErrorMessage(err error) string {
	if status, isStatus := err.(apierrors.APIStatus); isStatus {
		switch s := status.Status(); {
		case s.Reason == metav1.StatusReasonUnauthorized:
			return fmt.Sprintf("error: You must be logged in to the server (%s)", s.Message)
		case len(s.Reason) > 0:
			return fmt.Sprintf("Error from server (%s): %s", s.Reason, err.Error())
		default:
			return fmt.Sprintf("Error from server: %s", err.Error())
		}
	}

	if apierrors.IsUnexpectedObjectError(err) {
		return fmt.Sprintf("Server returned an unexpected response: %s", err.Error())
	}

	if t, isURL := err.(*url.Error); isURL {
		if strings.Contains(t.Err.Error(), "connection refused") {
			host := t.URL
			if server, err := url.Parse(t.URL); err == nil {
				host = server.Host
			}
			return fmt.Sprintf("The connection to the server %s was refused - did you specify the right host or port?", host)
		}
		return fmt.Sprintf("Unable to connect to the server: %v", t.Err)
	}

	return fmt.Sprintf("error: %v", err)
}
