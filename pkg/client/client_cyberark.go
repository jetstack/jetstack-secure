package client

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/internal/cyberark"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	"github.com/jetstack/preflight/pkg/logs"
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
// It then minimizes the snapshot to avoid uploading unnecessary data.
// It initializes a data upload client with the configured HTTP client and credentials,
// then uploads a snapshot.
// The supplied Options are not used by this publisher.
func (o *CyberArkClient) PostDataReadingsWithOptions(ctx context.Context, readings []*api.DataReading, _ Options) error {
	log := klog.FromContext(ctx)
	var snapshot dataupload.Snapshot
	if err := convertDataReadings(defaultExtractorFunctions, readings, &snapshot); err != nil {
		return fmt.Errorf("while converting data readings: %s", err)
	}

	// Minimize the snapshot to reduce size and improve privacy
	minimizeSnapshot(log.V(logs.Debug), &snapshot)

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

// minimizeSnapshot reduces the size of the snapshot by removing unnecessary data.
//
// This reduces the bandwidth used when uploading the snapshot to CyberArk,
// it reduces the storage used by CyberArk to store the snapshot, and
// it provides better privacy for the cluster being scanned; only the necessary
// data is included in the snapshot.
//
// This is a best-effort attempt to minimize the snapshot size. If an error occurs
// during analysis of a secret, the error is logged and the secret is kept in the
// snapshot (i.e., not excluded). Errors do not prevent the snapshot from being uploaded.
//
// It performs the following minimization steps:
//
//  1. Removal of non-clientauth TLS secrets: It filters out TLS secrets that do
//     not contain a client certificate. This is done to avoid uploading large
//     TLS secrets that are not relevant for the CyberArk Discovery and Context
//     service.
//
// TODO(wallrj): Remove more from the snapshot as we learn more about what
// resources the Discovery and Context service require.
func minimizeSnapshot(log logr.Logger, snapshot *dataupload.Snapshot) {
	originalSecretCount := len(snapshot.Secrets)
	filteredSecrets := make([]runtime.Object, 0, originalSecretCount)
	for _, secret := range snapshot.Secrets {
		if isExcludableSecret(log, secret) {
			continue
		}
		filteredSecrets = append(filteredSecrets, secret)
	}
	snapshot.Secrets = filteredSecrets
	log.Info("Minimized snapshot", "originalSecretCount", originalSecretCount, "filteredSecretCount", len(snapshot.Secrets))
}

// isExcludableSecret filters out TLS secrets that are definitely of no interest
// to CyberArk's Discovery and Context service, specifically TLS secrets that do
// not contain a client certificate.
//
// The Secret is kept if there is any doubt or if there is a problem decoding
// its contents.
//
// Secrets are obtained by a DynamicClient, so they have type
// *unstructured.Unstructured.
func isExcludableSecret(log logr.Logger, obj runtime.Object) bool {
	// Fast path: type assertion and kind/type checks
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.Info("Object is not a Unstructured", "type", fmt.Sprintf("%T", obj))
		return false
	}
	if unstructuredObj.GetKind() != "Secret" || unstructuredObj.GetAPIVersion() != "v1" {
		return false
	}

	log = log.WithValues("namespace", unstructuredObj.GetNamespace(), "name", unstructuredObj.GetName())
	dataMap, found, err := unstructured.NestedMap(unstructuredObj.Object, "data")
	if err != nil || !found {
		log.Info("Secret data missing or not a map")
		return false
	}

	secretType, found, err := unstructured.NestedString(unstructuredObj.Object, "type")
	if err != nil || !found {
		log.Info("Secret object has no type")
		return false
	}

	if corev1.SecretType(secretType) != corev1.SecretTypeTLS {
		log.Info("Secrets of this type are never excluded", "type", secretType)
		return false
	}

	return isExcludableTLSSecret(log, dataMap)
}

// isExcludableTLSSecret checks if a TLS Secret contains a client certificate.
// It returns true if the Secret is a TLS Secret and its tls.crt does not
// contain a client certificate.
func isExcludableTLSSecret(log logr.Logger, dataMap map[string]interface{}) bool {
	tlsCrtRaw, found := dataMap[corev1.TLSCertKey]
	if !found {
		log.Info("TLS Secret does not contain tls.crt key")
		return true
	}

	// Decode base64 if necessary (K8s secrets store data as base64-encoded strings)
	var tlsCrtBytes []byte
	switch v := tlsCrtRaw.(type) {
	case string:
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.Info("Failed to decode tls.crt base64", "error", err.Error())
			return true
		}
		tlsCrtBytes = decoded
	case []byte:
		tlsCrtBytes = v
	default:
		log.Info("tls.crt is not a string or byte slice", "type", fmt.Sprintf("%T", v))
		return true
	}

	// Parse PEM certificate chain
	hasClientCert := searchPEM(tlsCrtBytes, func(block *pem.Block) bool {
		if block.Type != "CERTIFICATE" || len(block.Bytes) == 0 {
			return false
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			log.Info("Failed to parse PEM block as X.509 certificate", "error", err.Error())
			return false
		}
		// Check if the certificate has the ClientAuth EKU
		return isClientCertificate(cert)
	})
	return !hasClientCert
}

// searchPEM parses the given PEM data and applies the visitor function to each
// PEM block found. If the visitor function returns true for any block, the search
// stops and searchPEM returns true. If no blocks cause the visitor to return true,
// searchPEM returns false.
func searchPEM(data []byte, visitor func(*pem.Block) bool) bool {
	if visitor == nil {
		return false
	}
	// Parse the PEM encoded certificate chain
	var block *pem.Block
	rest := data
	for {
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if visitor(block) {
			return true
		}
	}
	return false
}

// isClientCertificate checks if the given certificate is a client certificate
// by checking if it has the ClientAuth EKU.
func isClientCertificate(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	// Skip CA certificates
	if cert.IsCA {
		return false
	}
	// Check if the certificate has the ClientAuth EKU
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			return true
		}
	}
	return false
}
