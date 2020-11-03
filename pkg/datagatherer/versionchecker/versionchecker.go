package versionchecker

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jetstack/preflight/pkg/datagatherer"
	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	vcapi "github.com/jetstack/version-checker/pkg/api"
	vcclient "github.com/jetstack/version-checker/pkg/client"
	vcchecker "github.com/jetstack/version-checker/pkg/controller/checker"
	vcsearch "github.com/jetstack/version-checker/pkg/controller/search"
	vcversion "github.com/jetstack/version-checker/pkg/version"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Config is the configuration for a VersionChecker DataGatherer.
type Config struct {
	// the version checker dg will also gather pods and so has the same options
	// as the dynamic datagatherer
	Dynamic                      k8s.ConfigDynamic
	VersionCheckerClientOptions  vcclient.Options
	VersionCheckerCheckerOptions vcapi.Options
}

// validate validates the configuration.
func (c *Config) validate() error {
	// TODO
	return nil
}

// NewDataGatherer creates a new VersionChecker DataGatherer
func (c *Config) NewDataGatherer(ctx context.Context) (datagatherer.DataGatherer, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	// ensure that the k8s dg will always get pods
	c.Dynamic.GroupVersionResource = schema.GroupVersionResource{
		Group: "", Version: "v1", Resource: "pods",
	}

	// create the k8s DataGatherer to use when collecting pods
	dynamicDg, err := c.Dynamic.NewDataGatherer(ctx)
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

	// dg wraps version checker and dynamic client to request pods
	return &DataGatherer{
		ctx:                   ctx,
		config:                c,
		dynamicDg:             dynamicDg,
		versionChecker:        vcchecker.New(search),
		versionCheckerLog:     log,
		versionCheckerOptions: c.VersionCheckerCheckerOptions,
	}, nil
}

// DataGatherer is a VersionChecker DataGatherer
type DataGatherer struct {
	ctx                   context.Context
	config                *Config
	dynamicDg             datagatherer.DataGatherer
	versionChecker        *vcchecker.Checker
	versionCheckerLog     *logrus.Entry
	versionCheckerOptions vcapi.Options
}

// PodResult wraps a pod and a version checker result for an image found in one
// of the containers for that pod
type PodResult struct {
	Pod    v1.Pod
	Result *vcchecker.Result
}

// Fetch retrieves cluster information from GKE.
func (g *DataGatherer) Fetch() (interface{}, error) {
	rawPods, err := g.dynamicDg.Fetch()
	if err != nil {
		return nil, err
	}

	pods, ok := rawPods.(*unstructured.UnstructuredList)
	if !ok {
		return nil, fmt.Errorf("failed to parse pods loaded from DataGatherer")
	}

	var results []PodResult
	for _, v := range pods.Items {
		var pod v1.Pod
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(v.Object, &pod)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod from unstructured data")
		}

		// allContainers will contain a list of containers and init containers,
		// they will be checked in the same way
		var allContainers []v1.Container
		allContainers = append(allContainers, pod.Spec.Containers...)
		allContainers = append(allContainers, pod.Spec.InitContainers...)

		for _, c := range allContainers {
			result, err := g.versionChecker.Container(g.ctx, g.versionCheckerLog, &pod, &c, &g.versionCheckerOptions)
			if err != nil {
				return nil, fmt.Errorf("failed to check image for container: %s", err)
			}

			results = append(results, PodResult{Pod: pod, Result: result})
		}
	}

	return results, nil
}
