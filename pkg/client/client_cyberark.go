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
// It initializes a data upload client with the configured HTTP client and credentials,
// then uploads a snapshot.
// The supplied Options are not used by this publisher.
func (o *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	cfg, err := o.configLoader()
	if err != nil {
		return err
	}
	datauploadClient, err := cyberark.NewDatauploadClient(ctx, o.httpClient, cfg)
	if err != nil {
		return fmt.Errorf("while initializing data upload client: %s", err)
	}
	var snapshot dataupload.Snapshot
	if err := ConvertDataReadingsToCyberarkSnapshot(readings, &snapshot); err != nil {
		return fmt.Errorf("while converting data readings: %s", err)
	}
	// Temporary hard coded cluster ID.
	// TODO(wallrj): The clusterID will eventually be extracted from the supplied readings.
	snapshot.ClusterID = "success-cluster-id"

	err = datauploadClient.PutSnapshot(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("while uploading snapshot: %s", err)
	}
	return nil
}

// extractServerVersionFromReading converts the opaque data from a DiscoveryData
// data reading to allow access to the Kubernetes version fields within.
func extractServerVersionFromReading(reading *api.DataReading, target *string) error {
	data, ok := reading.Data.(*api.DiscoveryData)
	if !ok {
		return fmt.Errorf(
			"programmer mistake: the DataReading must have data type *api.DiscoveryData. "+
				"This DataReading (%s) has data type %T", reading.DataGatherer, reading.Data)
	}
	if data.ServerVersion == nil {
		return nil
	}
	*target = data.ServerVersion.GitVersion
	return nil
}

// extractResourceListFromReading converts the opaque data from a DynamicData
// data reading to runtime.Object resources, to allow access to the metadata and
// other kubernetes API fields.
func extractResourceListFromReading(reading *api.DataReading, target *[]runtime.Object) error {
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

var expectedGathererNames = []string{
	"ark/secrets",
	"ark/serviceaccounts",
	"ark/roles",
	"ark/clusterroles",
	"ark/rolebindings",
	"ark/clusterrolebindings",
	"ark/jobs",
	"ark/cronjobs",
	"ark/deployments",
	"ark/statefulsets",
	"ark/daemonsets",
	"ark/pods",
}

// ConvertDataReadingsToCyberarkSnapshot converts a list of DataReadings to a CyberArk Snapshot.
// It extracts the Kubernetes version from the "ark/discovery" DataReading and
// collects resources from other DataReadings based on their DataGatherer names.
// If any required data is missing or cannot be converted, an error is returned.
func ConvertDataReadingsToCyberarkSnapshot(
	readings []*api.DataReading,
	snapshot *dataupload.Snapshot,
) error {
	allDataGathererNames := make([]string, len(readings))
	unhandledDataGatherers := sets.New[string]()
	expectedDataGatherers := sets.New[string](expectedGathererNames...)
	for i, reading := range readings {
		dataGathererName := reading.DataGatherer
		allDataGathererNames[i] = dataGathererName
		var err error
		switch reading.DataGatherer {
		case "ark/discovery":
			err = extractServerVersionFromReading(reading, &snapshot.K8SVersion)
		case "ark/secrets":
			err = extractResourceListFromReading(reading, &snapshot.Secrets)
		case "ark/serviceaccounts":
			err = extractResourceListFromReading(reading, &snapshot.ServiceAccounts)
		case "ark/roles":
			err = extractResourceListFromReading(reading, &snapshot.Roles)
		case "ark/clusterroles":
			err = extractResourceListFromReading(reading, &snapshot.ClusterRoles)
		case "ark/rolebindings":
			err = extractResourceListFromReading(reading, &snapshot.RoleBindings)
		case "ark/clusterrolebindings":
			err = extractResourceListFromReading(reading, &snapshot.ClusterRoleBindings)
		case "ark/jobs":
			err = extractResourceListFromReading(reading, &snapshot.Jobs)
		case "ark/cronjobs":
			err = extractResourceListFromReading(reading, &snapshot.CronJobs)
		case "ark/deployments":
			err = extractResourceListFromReading(reading, &snapshot.Deployments)
		case "ark/statefulsets":
			err = extractResourceListFromReading(reading, &snapshot.Statefulsets)
		case "ark/daemonsets":
			err = extractResourceListFromReading(reading, &snapshot.Daemonsets)
		case "ark/pods":
			err = extractResourceListFromReading(reading, &snapshot.Pods)
		default:
			unhandledDataGatherers.Insert(dataGathererName)
		}
		if err != nil {
			return fmt.Errorf("while extracting data reading %s: %s", dataGathererName, err)
		}
	}
	allDataGatherers := sets.New[string](allDataGathererNames...)
	missingDataGatherers := expectedDataGatherers.Difference(allDataGatherers)
	if missingDataGatherers.Len() > 0 || unhandledDataGatherers.Len() > 0 {
		return fmt.Errorf(
			"Unexpected data gatherers. missing: %v, unhandled: %v",
			sets.List(missingDataGatherers),
			sets.List(unhandledDataGatherers),
		)
	}
	return nil
}
