package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

	snapshot, err := ConvertDataReadingsToCyberarkSnapshot(readings)
	if err != nil {
		return fmt.Errorf("while converting data readings: %s", err)
	}

	err = datauploadClient.PutSnapshot(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("while uploading snapshot: %s", err)
	}
	return nil
}

type resourceData map[string][]*unstructured.Unstructured

// The names of Datagatherers which have the data to populate the Cyberark
// Snapshot mapped to the key in the Cyberark snapshot.
var gathererNameToResourceDataKeyMap = map[string]string{
	"ark/secrets":             "secrets",
	"ark/serviceaccounts":     "serviceaccounts",
	"ark/roles":               "roles",
	"ark/clusterroles":        "clusterroles",
	"ark/rolebindings":        "rolebindings",
	"ark/clusterrolebindings": "clusterrolebindings",
	"ark/jobs":                "jobs",
	"ark/cronjobs":            "cronjobs",
	"ark/deployments":         "deployments",
	"ark/statefulsets":        "statefulsets",
	"ark/daemonsets":          "daemonsets",
	"ark/pods":                "pods",
}

// extractClusterUIDFromReading converts the opaque data from a DynamicData
// reading to Unstructured Namespace resources, and finds the UID of the
// `kube-system` namespace.
// This UID can be used as a unique identifier for the Kubernetes cluster.
// - https://venafi.slack.com/archives/C04SQR5DAD7/p1747825325264979
// - https://github.com/kubernetes/kubernetes/issues/77487#issuecomment-489786023
func extractClusterUIDFromReading(reading *api.DataReading) (string, error) {
	resources, err := extractResourceListFromReading(reading)
	if err != nil {
		return "", err
	}
	for _, resource := range resources {
		if resource.GetName() == "kube-system" {
			return string(resource.GetUID()), nil
		}
	}
	return "", fmt.Errorf("kube-system namespace UID not found in data reading: %v", reading)
}

// extractServerVersionFromReading converts the opaque data from a DiscoveryData
// data reding to allow access to the Kubernetes version fields within.
func extractServerVersionFromReading(reading *api.DataReading) (string, error) {
	data, ok := reading.Data.(*api.DiscoveryData)
	if !ok {
		return "", fmt.Errorf("failed to convert data: %s", reading.DataGatherer)
	}
	if data.ServerVersion == nil {
		return "unknown", nil
	}
	return data.ServerVersion.GitVersion, nil
}

// extractResourceListFromReading converts the opaque data from a DynamicData
// data reading to Unstructured resources, to allow access to the metadata and
// other kubernetes API fields.
func extractResourceListFromReading(reading *api.DataReading) ([]*unstructured.Unstructured, error) {
	data, ok := reading.Data.(*api.DynamicData)
	if !ok {
		return nil, fmt.Errorf("failed to convert data: %s", reading.DataGatherer)
	}
	items := data.Items
	resources := make([]*unstructured.Unstructured, len(items))
	for i, item := range items {
		if resource, ok := item.Resource.(*unstructured.Unstructured); ok {
			resources[i] = resource
		} else {
			return nil, fmt.Errorf("failed to convert resource: %#v", item)
		}
	}
	return resources, nil
}

// ConvertDataReadingsToCyberarkSnapshot converts DataReadings to the Cyberark
// Snapshot format.
func ConvertDataReadingsToCyberarkSnapshot(
	readings []*api.DataReading,
) (s dataupload.Snapshot, _ error) {
	k8sVersion := ""
	clusterID := ""
	resourceData := resourceData{}
	for _, reading := range readings {
		if reading.DataGatherer == "ark/discovery" {
			var err error
			k8sVersion, err = extractServerVersionFromReading(reading)
			if err != nil {
				return s, fmt.Errorf("while extracting server version from data-reading: %s", err)
			}
		}
		if reading.DataGatherer == "ark/namespaces" {
			var err error
			clusterID, err = extractClusterUIDFromReading(reading)
			if err != nil {
				return s, fmt.Errorf("while extracting cluster UID from data-reading: %s", err)
			}
		}
		if key, found := gathererNameToResourceDataKeyMap[reading.DataGatherer]; found {
			resources, err := extractResourceListFromReading(reading)
			if err != nil {
				return s, fmt.Errorf("while extracting resource list from data-reading: %s", err)
			}
			resourceData[key] = append(resourceData[key], resources...)
		}
	}
	if clusterID == "" {
		return s, errors.New("failed to compute a clusterID from the data-readings")
	}
	return dataupload.Snapshot{
		AgentVersion:        version.PreflightVersion,
		K8SVersion:          k8sVersion,
		ClusterID:           clusterID,
		Secrets:             resourceData["secrets"],
		ServiceAccounts:     resourceData["serviceaccounts"],
		Roles:               resourceData["roles"],
		ClusterRoles:        resourceData["clusterroles"],
		RoleBindings:        resourceData["rolebindings"],
		ClusterRoleBindings: resourceData["clusterrolebindings"],
		Jobs:                resourceData["jobs"],
		CronJobs:            resourceData["cronjobs"],
		Deployments:         resourceData["deployments"],
		Statefulsets:        resourceData["statefulsets"],
		Daemonsets:          resourceData["daemonsets"],
		Pods:                resourceData["pods"],
	}, nil
}
