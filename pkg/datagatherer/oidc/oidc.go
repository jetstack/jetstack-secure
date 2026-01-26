package oidc

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/kubeconfig"
)

// OIDCDiscovery contains the configuration for the k8s-discovery data-gatherer
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

// DataGathererOIDC stores the config for a k8s-discovery datagatherer
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
func (g *DataGathererOIDC) Fetch() (any, int, error) {
	ctx := context.Background()

	oidcResponse, oidcErr := g.fetchOIDCConfig(ctx)
	jwksResponse, jwksErr := g.fetchJWKS(ctx)

	errToString := func(err error) string {
		if err != nil {
			return err.Error()
		}
		return ""
	}

	return api.OIDCDiscoveryData{
		OIDCConfig:      oidcResponse,
		OIDCConfigError: errToString(oidcErr),
		JWKS:            jwksResponse,
		JWKSError:       errToString(jwksErr),
	}, 1 /* we have 1 result, so return 1 as count */, nil
}

func (g *DataGathererOIDC) fetchOIDCConfig(ctx context.Context) (map[string]any, error) {
	// Fetch the OIDC discovery document from the well-known endpoint.
	bytes, err := g.cl.Get().AbsPath("/.well-known/openid-configuration").Do(ctx).Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get OIDC discovery document: %v", err)
	}

	var oidcResponse map[string]any
	if err := json.Unmarshal(bytes, &oidcResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OIDC discovery document: %v", err)
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
	bytes, err := g.cl.Get().AbsPath("/openid/v1/jwks").Do(ctx).Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS from jwks_uri: %v", err)
	}

	var jwksResponse map[string]any
	if err := json.Unmarshal(bytes, &jwksResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWKS response: %v", err)
	}

	return jwksResponse, nil
}
