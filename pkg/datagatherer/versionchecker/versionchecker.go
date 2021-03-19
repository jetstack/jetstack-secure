package versionchecker

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jetstack/preflight/api"
	vcapi "github.com/jetstack/version-checker/pkg/api"
	vcchecker "github.com/jetstack/version-checker/pkg/checker"
	vcarchitecture "github.com/jetstack/version-checker/pkg/checker/architecture"
	vcsearch "github.com/jetstack/version-checker/pkg/checker/search"
	vcversion "github.com/jetstack/version-checker/pkg/checker/version"
	vcclient "github.com/jetstack/version-checker/pkg/client"
	vcselfhosted "github.com/jetstack/version-checker/pkg/client/selfhosted"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
)

const (
	// these are the keys used to look the file paths up from the supplied
	// config
	gcrTokenKey = "token"

	acrUsernameKey     = "username"
	acrPasswordKey     = "password"
	acrRefreshTokenKey = "refresh_token"

	ecrAccessKeyIdKey     = "access_key_id"
	ecrSecretAccessKeyKey = "secret_access_key"
	ecrSessionTokenKey    = "session_token"

	dockerUsernameKey = "username"
	dockerPasswordKey = "password"
	dockerTokenKey    = "token"

	quayTokenKey = "token"

	selfhostedUsernameKey = "username"
	selfhostedPasswordKey = "password"
	selfhostedBearerKey   = "bearer"
	selfhostedHostKey     = "host"
)

// Config is the configuration for a VersionChecker DataGatherer.
type Config struct {
	// the version checker dg will also gather pods and so has the same options
	// as the dynamic datagatherer
	DynamicPod k8s.ConfigDynamic
	// the nodes information is also gathered by the version checker datagatherer
	DynamicNode                 k8s.ConfigDynamic
	VersionCheckerClientOptions vcclient.Options
	// Currently unused, but keeping to allow future config of VersionChecker
	VersionCheckerCheckerOptions vcapi.Options
}

// UnmarshalYAML unmarshals the ConfigDynamic resolving GroupVersionResource.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	aux := struct {
		Dynamic struct {
			KubeConfigPath    string   `yaml:"kubeconfig"`
			ExcludeNamespaces []string `yaml:"exclude-namespaces"`
			IncludeNamespaces []string `yaml:"include-namespaces"`
		} `yaml:"k8s"`
		Registries []struct {
			Kind   string            `yaml:"kind"`
			Params map[string]string `yaml:"params"`
		} `yaml:"registries"`
	}{}
	err := unmarshal(&aux)
	if err != nil {
		return fmt.Errorf("failed to unmarshal version checker config: %s", err)
	}

	c.DynamicPod.KubeConfigPath = aux.Dynamic.KubeConfigPath
	c.DynamicPod.ExcludeNamespaces = aux.Dynamic.ExcludeNamespaces
	c.DynamicPod.IncludeNamespaces = aux.Dynamic.IncludeNamespaces
	// gvr must be pods for the version checker dg
	c.DynamicPod.GroupVersionResource.Group = ""
	c.DynamicPod.GroupVersionResource.Version = "v1"
	c.DynamicPod.GroupVersionResource.Resource = "pods"
	// node dynamic dg
	c.DynamicNode.KubeConfigPath = aux.Dynamic.KubeConfigPath
	c.DynamicNode.ExcludeNamespaces = []string{}
	c.DynamicNode.IncludeNamespaces = []string{}
	c.DynamicNode.GroupVersionResource.Group = ""
	c.DynamicNode.GroupVersionResource.Version = "v1"
	c.DynamicNode.GroupVersionResource.Resource = "nodes"

	c.VersionCheckerClientOptions.Selfhosted = map[string]*vcselfhosted.Options{}
	registryKindCounts := map[string]int{}
	for i, v := range aux.Registries {
		registryKindCounts[v.Kind]++
		switch v.Kind {
		case "gcr":
			data, err := loadKeysFromPaths([]string{gcrTokenKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.GCR.Token = data[gcrTokenKey]
		case "acr":
			data, err := loadKeysFromPaths([]string{acrUsernameKey, acrPasswordKey, acrRefreshTokenKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.ACR.Username = data[acrUsernameKey]
			c.VersionCheckerClientOptions.ACR.Password = data[acrPasswordKey]
			c.VersionCheckerClientOptions.ACR.RefreshToken = data[acrRefreshTokenKey]
		case "ecr":
			data, err := loadKeysFromPaths([]string{ecrAccessKeyIdKey, ecrSecretAccessKeyKey, ecrSessionTokenKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.ECR.AccessKeyID = data[ecrAccessKeyIdKey]
			c.VersionCheckerClientOptions.ECR.SecretAccessKey = data[ecrSecretAccessKeyKey]
			c.VersionCheckerClientOptions.ECR.SessionToken = data[ecrSessionTokenKey]
		case "docker":
			data, err := loadKeysFromPaths([]string{dockerUsernameKey, dockerPasswordKey, dockerTokenKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.Docker.Username = data[dockerUsernameKey]
			c.VersionCheckerClientOptions.Docker.Password = data[dockerPasswordKey]
			c.VersionCheckerClientOptions.Docker.Token = data[dockerTokenKey]
		case "quay":
			data, err := loadKeysFromPaths([]string{quayTokenKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.Quay.Token = data[quayTokenKey]
		case "selfhosted":
			// currently, version checker only supports multiple selfhosted registries
			data, err := loadKeysFromPaths([]string{selfhostedUsernameKey, selfhostedPasswordKey, selfhostedHostKey, selfhostedBearerKey}, v.Params)
			if err != nil {
				return fmt.Errorf("failed to load params for %s registry %d/%d: %s", v.Kind, i+1, len(aux.Registries), err)
			}

			opts := vcselfhosted.Options{
				Username: data[selfhostedUsernameKey],
				Password: data[selfhostedPasswordKey],
				Bearer:   data[selfhostedBearerKey],
				Host:     data[selfhostedHostKey],
			}

			if len(opts.Host) == 0 {
				return fmt.Errorf("failed to init selfhosted dg, host is required (registry %d/%d): %s", i+1, len(aux.Registries), err)
			}

			parsedURL, err := url.Parse(opts.Host)
			if err != nil {
				return fmt.Errorf("failed to parse host %s (registry %d/%d): %s", opts.Host, i+1, len(aux.Registries), err)
			}

			c.VersionCheckerClientOptions.Selfhosted[parsedURL.Host] = &opts
		default:
			return fmt.Errorf("registry %d/%d was an unknown kind (%s)", i+1, len(aux.Registries), v.Kind)
		}
	}

	// this is only needed while version checker only supports one registry of
	// each kind. Using an array of registries in the config allows us to
	// support many in future without changing the config format.
	for k, v := range registryKindCounts {
		if v > 1 && k != "selfhosted" {
			return fmt.Errorf("found %d registries of kind %s, only 1 is supported", v, k)
		}
	}

	return nil
}

func loadKeysFromPaths(keys []string, params map[string]string) (map[string]string, error) {
	requiredKeys := map[string]bool{}
	for _, v := range keys {
		requiredKeys[v] = false
	}

	loadedData := map[string]string{}
	for _, k := range keys {
		path := params[k]
		if path == "" {
			// don't try to load unset secrets. version-checker will fail if
			// config is missing
			continue
		}

		b, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file for %s at %s: %s", k, path, err)
		}
		loadedData[k] = strings.TrimSpace(string(b))
	}

	return loadedData, nil
}

// NewDataGatherer creates a new VersionChecker DataGatherer
func (c *Config) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	// create the k8s DataGatherer to use when collecting pods
	podDynamicDg, err := c.DynamicPod.NewDataGatherer(ctx)
	if err != nil {
		return nil, err
	}
	// create a data gatherer to use to collect nodes architecture information
	nodeDynamicDg, err := c.DynamicNode.NewDataGatherer(ctx)
	if err != nil {
		return nil, err
	}

	// configure version checker parameters
	vclog := logrus.New()
	vclog.SetOutput(os.Stdout)
	log := logrus.NewEntry(vclog)
	imageClient, err := vcclient.New(ctx, log, c.VersionCheckerClientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to setup version checker image registry clients: %s", err)
	}
	timeout := 30 * time.Minute // this is the default used in version checker
	search := vcsearch.New(
		log,
		timeout,
		vcversion.New(log, imageClient, timeout),
	)
	architecture := vcarchitecture.New()

	// dg wraps version checker and dynamic client to request pods
	return &DataGatherer{
		ctx:                   ctx,
		config:                c,
		podDynamicDg:          podDynamicDg,
		nodeDynamicDg:         nodeDynamicDg,
		nodeArchitecture:      architecture,
		versionChecker:        vcchecker.New(search, architecture),
		versionCheckerLog:     log,
		versionCheckerOptions: c.VersionCheckerCheckerOptions,
	}, nil
}

// DataGatherer is a VersionChecker DataGatherer
type DataGatherer struct {
	ctx                   context.Context
	config                *Config
	podDynamicDg          datagatherer.DataGatherer
	nodeDynamicDg         datagatherer.DataGatherer
	nodeArchitecture      *vcarchitecture.NodeMap
	versionChecker        *vcchecker.Checker
	versionCheckerLog     *logrus.Entry
	versionCheckerOptions vcapi.Options
}

// PodResult wraps a pod and a version checker result for an image found in one
// of the containers for that pod. Exported so the backend can destructure
// json.
type PodResult struct {
	Pod     v1.Pod            `json:"pod"`
	Results []containerResult `json:"results"`
}

type containerResult struct {
	ContainerName   string            `json:"container_name"`
	InitContinainer bool              `json:"init_container"`
	Result          *vcchecker.Result `json:"result"`
}

// Run starts the version checker data gatherer's dynamic informers for resource collection.
// Returns error if the pod and node data gatherers haven't been correctly initialized
func (g *DataGatherer) Run(stopCh <-chan struct{}) error {
	// start dynamic dynamic data gatherers informes
	if err := g.podDynamicDg.Run(stopCh); err != nil {
		return err
	}
	return g.nodeDynamicDg.Run(stopCh)
}

// WaitForCacheSync waits for the data gatherer's informers cache to sync before collecting the resources.
func (g *DataGatherer) WaitForCacheSync(stopCh <-chan struct{}) error {
	if err := g.podDynamicDg.WaitForCacheSync(stopCh); err != nil {
		return err
	}
	return g.nodeDynamicDg.WaitForCacheSync(stopCh)
}

func (g *DataGatherer) Delete() error {
	if g.podDynamicDg != nil {
		if err := g.podDynamicDg.Delete(); err != nil {
			return err
		}
	}
	if g.nodeDynamicDg != nil {
		if err := g.nodeDynamicDg.Delete(); err != nil {
			return err
		}
	}
	return nil
}

// Fetch retrieves cluster information from GKE.
func (g *DataGatherer) Fetch() (interface{}, error) {
	// Get nodes information to update version-checker architecture structure
	rawNodes, err := g.nodeDynamicDg.Fetch()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodes: %v", err)
	}

	nodeItems, ok := rawNodes.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse nodes loaded from DataGatherer, is not map[string]interface{}")
	}

	var nodes []*api.GatheredResource = []*api.GatheredResource{}
	if items, ok := nodeItems["items"]; ok {
		nodes, ok = items.([]*api.GatheredResource)
		if !ok {
			return nil, fmt.Errorf("failed to parse nodes loaded from DataGatherer, is not []*api.GatheredResource")
		}
	}

	for _, v := range nodes {
		var node v1.Node
		resource, ok := v.Resource.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("failed to parse node resource")
		}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, &node)
		if err != nil {
			return nil, fmt.Errorf("failed to parse node from unstructured data: %v", err)
		}
		// update version-checker's internal representation of the current cluster's nodes,
		// to correctly select the right OS and Architecture for the images
		if err = g.nodeArchitecture.Add(&node); err != nil {
			return nil, fmt.Errorf("failed to add node to version-checker architecture structure")
		}
	}

	rawPods, err := g.podDynamicDg.Fetch()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pods: %v", err)
	}

	podItems, ok := rawPods.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse pods loaded from DataGatherer, is not map[string]interface{}")
	}

	var pods []*api.GatheredResource = []*api.GatheredResource{}
	if items, ok := podItems["items"]; ok {
		pods, ok = items.([]*api.GatheredResource)
		if !ok {
			return nil, fmt.Errorf("failed to parse pods loaded from DataGatherer, is not []*api.GatheredResource")
		}
	}

	var results []PodResult
	for _, v := range pods {
		var pod v1.Pod
		resource, ok := v.Resource.(*unstructured.Unstructured)
		if !ok {
			return nil, fmt.Errorf("failed to parse node resource")
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(resource.Object, &pod)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod from unstructured data: %v", err)
		}

		var allContainers []v1.Container
		var isInitContainer []bool
		for _, c := range pod.Spec.Containers {
			allContainers = append(allContainers, c)
			isInitContainer = append(isInitContainer, false)
		}
		for _, c := range pod.Spec.InitContainers {
			allContainers = append(allContainers, c)
			isInitContainer = append(isInitContainer, true)
		}

		var containerResults []containerResult
		for i, c := range allContainers {
			result, err := g.versionChecker.Container(g.ctx, g.versionCheckerLog, &pod, &c, &g.versionCheckerOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to check image for container: %s", err)
			}

			containerResults = append(
				containerResults,
				containerResult{
					ContainerName:   c.Name,
					InitContinainer: isInitContainer[i],
					Result:          result,
				},
			)
		}

		results = append(results, PodResult{Pod: pod, Results: containerResults})
	}

	return results, nil
}
