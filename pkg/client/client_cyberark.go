package client

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/version"
)

// CyberArkClient is a client for publishing data readings to CyberArk's discoverycontext API.
type CyberArkClient struct {
	configLoader cyberark.ClientConfigLoader
	httpClient   *http.Client
}

var _ Client = &CyberArkClient{}

// NewCyberArk initializes a CyberArk client using configuration from environment variables.
// It requires an HTTP client to be provided, which will be used for making requests.
// The environment variables ARK_SUBDOMAIN, ARK_USERNAME, and ARK_SECRET must be set for authentication.
// If the configuration is invalid or missing, an error is returned.
func NewCyberArk(httpClient *http.Client) (*CyberArkClient, error) {
	configLoader := cyberark.LoadClientConfigFromEnvironment
	_, err := configLoader()
	if err != nil {
		return nil, err
	}
	return &CyberArkClient{
		configLoader: configLoader,
		httpClient:   httpClient,
	}, nil
}

// PostDataReadingsWithOptions uploads data readings to CyberArk.
// It converts the supplied data readings into a snapshot format expected by CyberArk.
// It initializes a data upload client with the configured HTTP client and credentials,
// then uploads a snapshot.
// The supplied Options are not used by this publisher.
func (o *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	var snapshot dataupload.Snapshot
	if err := convertDataReadings(defaultExtractorFunctions, readings, &snapshot); err != nil {
		return fmt.Errorf("while converting data readings: %s", err)
	}
	snapshot.AgentVersion = version.PreflightVersion

	cfg, err := o.configLoader()
	if err != nil {
		return err
	}
	datauploadClient, err := cyberark.NewDatauploadClient(ctx, o.httpClient, cfg)
	if err != nil {
		return fmt.Errorf("while initializing data upload client: %s", err)
	}

	err = datauploadClient.PutSnapshot(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("while uploading snapshot: %s", err)
	}
	return nil
}

// extractClusterIDAndServerVersionFromReading converts the opaque data from a DiscoveryData
// data reading to allow access to the Kubernetes version fields within.
func extractClusterIDAndServerVersionFromReading(reading *api.DataReading, target *dataupload.Snapshot) error {
	if reading == nil {
		return fmt.Errorf("programmer mistake: the DataReading must not be nil")
	}
	data, ok := reading.Data.(*api.DiscoveryData)
	if !ok {
		return fmt.Errorf(
			"programmer mistake: the DataReading must have data type *api.DiscoveryData. "+
				"This DataReading (%s) has data type %T", reading.DataGatherer, reading.Data)
	}
	target.ClusterID = data.ClusterID
	if data.ServerVersion != nil {
		target.K8SVersion = data.ServerVersion.GitVersion
	}
	return nil
}

// extractResourceListFromReading converts the opaque data from a DynamicData
// data reading to runtime.Object resources, to allow access to the metadata and
// other kubernetes API fields.
func extractResourceListFromReading(reading *api.DataReading, target *[]runtime.Object) error {
	if reading == nil {
		return fmt.Errorf("programmer mistake: the DataReading must not be nil")
	}
	data, ok := reading.Data.(*api.DynamicData)
	if !ok {
		return fmt.Errorf(
			"programmer mistake: the DataReading must have data type *api.DynamicData. "+
				"This DataReading (%s) has data type %T", reading.DataGatherer, reading.Data)
	}
	resources := make([]runtime.Object, len(data.Items))
	for i, item := range data.Items {
		if resource, ok := item.Resource.(runtime.Object); ok {
			resources[i] = resource
		} else {
			return fmt.Errorf(
				"programmer mistake: the DynamicData items must have Resource type runtime.Object. "+
					"This item (%d) has Resource type %T", i, item.Resource)
		}
	}
	*target = resources
	return nil
}

var defaultExtractorFunctions = map[string]func(*api.DataReading, *dataupload.Snapshot) error{
	"ark/discovery": extractClusterIDAndServerVersionFromReading,
	"ark/secrets": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Secrets)
	},
	"ark/serviceaccounts": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.ServiceAccounts)
	},
	"ark/roles": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Roles)
	},
	"ark/clusterroles": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.ClusterRoles)
	},
	"ark/rolebindings": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.RoleBindings)
	},
	"ark/clusterrolebindings": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.ClusterRoleBindings)
	},
	"ark/jobs": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Jobs)
	},
	"ark/cronjobs": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.CronJobs)
	},
	"ark/deployments": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Deployments)
	},
	"ark/statefulsets": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Statefulsets)
	},
	"ark/daemonsets": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Daemonsets)
	},
	"ark/pods": func(r *api.DataReading, s *dataupload.Snapshot) error {
		return extractResourceListFromReading(r, &s.Pods)
	},
}

// convertDataReadings processes a list of DataReadings using the provided
// extractor functions to populate the fields of the target snapshot.
// It ensures that all expected data gatherers are handled and that there are
// no unhandled data gatherers. If any discrepancies are found, or if any
// extractor function returns an error, it returns an error.
// The extractorFunctions map should contain functions for each expected
// DataGatherer name, which will be called with the corresponding DataReading
// and the target snapshot to populate the relevant fields.
func convertDataReadings(
	extractorFunctions map[string]func(*api.DataReading, *dataupload.Snapshot) error,
	readings []*api.DataReading,
	target *dataupload.Snapshot,
) error {
	expectedDataGatherers := sets.KeySet(extractorFunctions)
	unhandledDataGatherers := sets.New[string]()
	missingDataGatherers := expectedDataGatherers.Clone()
	for _, reading := range readings {
		dataGathererName := reading.DataGatherer
		extractFunc, found := extractorFunctions[dataGathererName]
		if !found {
			unhandledDataGatherers.Insert(dataGathererName)
			continue
		}
		missingDataGatherers.Delete(dataGathererName)
		// Call the extractor function to populate the relevant field in the target snapshot.
		if err := extractFunc(reading, target); err != nil {
			return fmt.Errorf("while extracting data reading %s: %s", dataGathererName, err)
		}
	}
	if missingDataGatherers.Len() > 0 || unhandledDataGatherers.Len() > 0 {
		return fmt.Errorf(
			"unexpected data gatherers, missing: %v, unhandled: %v",
			sets.List(missingDataGatherers),
			sets.List(unhandledDataGatherers),
		)
	}
	return nil
}
