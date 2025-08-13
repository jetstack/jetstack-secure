package dataupload

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/version"
)

type resourceData map[string][]*unstructured.Unstructured

// snapshot is the JSON that the CyberArk Discovery and Context API expects to
// be uploaded to the AWS presigned URL.
type snapshot struct {
	AgentVersion    string                       `json:"agent_version"`
	ClusterID       string                       `json:"cluster_id"`
	K8SVersion      string                       `json:"k8s_version"`
	Secrets         []*unstructured.Unstructured `json:"secrets"`
	ServiceAccounts []*unstructured.Unstructured `json:"service_accounts"`
	Roles           []*unstructured.Unstructured `json:"roles"`
	RoleBindings    []*unstructured.Unstructured `json:"role_bindings"`
}

// The names of Datagatherers which have the data to populate the Cyberark
// Snapshot mapped to the key in the Cyberark snapshot.
var gathererNameToResourceDataKeyMap = map[string]string{
	"ark/secrets":             "secrets",
	"ark/serviceaccounts":     "serviceaccounts",
	"ark/roles":               "roles",
	"ark/clusterroles":        "roles",
	"ark/rolebindings":        "rolebindings",
	"ark/clusterrolebindings": "rolebindings",
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

// convertDataReadingsToCyberarkSnapshot converts DataReadings to the Cyberark
// Snapshot format.
// The ClusterUID is the UID of the kube-system namespace, which is assumed to
// be unique to the cluster and assumed to never change.
func convertDataReadingsToCyberarkSnapshot(
	readings []*api.DataReading,
) (*snapshot, error) {
	k8sVersion := ""
	clusterID := ""
	resourceData := resourceData{}
	for _, reading := range readings {
		if reading.DataGatherer == "ark/discovery" {
			var err error
			k8sVersion, err = extractServerVersionFromReading(reading)
			if err != nil {
				return nil, fmt.Errorf("while extracting server version from data-reading: %s", err)
			}
		}

		if reading.DataGatherer == "ark/namespaces" {
			var err error
			clusterID, err = extractClusterUIDFromReading(reading)
			if err != nil {
				return nil, fmt.Errorf("while extracting cluster UID from data-reading: %s", err)
			}
		}

		if key, found := gathererNameToResourceDataKeyMap[reading.DataGatherer]; found {
			resources, err := extractResourceListFromReading(reading)
			if err != nil {
				return nil, fmt.Errorf("while extracting resource list from data-reading: %s", err)
			}
			resourceData[key] = append(resourceData[key], resources...)
		}
	}
	if clusterID == "" {
		return nil, errors.New("failed to compute a clusterID from the data-readings")
	}
	return &snapshot{
		AgentVersion:    version.PreflightVersion,
		K8SVersion:      k8sVersion,
		ClusterID:       clusterID,
		Secrets:         resourceData["secrets"],
		ServiceAccounts: resourceData["serviceaccounts"],
		Roles:           resourceData["roles"],
		RoleBindings:    resourceData["rolebindings"],
	}, nil
}
