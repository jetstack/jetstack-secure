package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/internal/cyberark/dataupload"
	preflightversion "github.com/jetstack/preflight/pkg/version"
)

// TestBaseSnapshotFromOptions tests the baseSnapshotFromOptions function.
func TestBaseSnapshotFromOptions(t *testing.T) {
	type testCase struct {
		name    string
		options Options
		want    dataupload.Snapshot
	}
	tests := []testCase{
		{
			name: "ClusterName and ClusterDescription are used, OrgID and ClusterID",
			options: Options{
				OrgID:              "unused-org-id",
				ClusterID:          "unused-cluster-id",
				ClusterName:        "some-cluster-name",
				ClusterDescription: "some-cluster-description",
			},
			want: dataupload.Snapshot{
				ClusterName:        "some-cluster-name",
				ClusterDescription: "some-cluster-description",
				AgentVersion:       preflightversion.PreflightVersion,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := baseSnapshotFromOptions(tc.options)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestExtractServerVersionFromReading tests the extractServerVersionFromReading function.
func TestExtractServerVersionFromReading(t *testing.T) {
	type testCase struct {
		name             string
		reading          *api.DataReading
		expectedSnapshot dataupload.Snapshot
		expectError      string
	}
	tests := []testCase{
		{
			name:        "nil reading",
			expectError: `programmer mistake: the DataReading must not be nil`,
		},
		{
			name: "nil data",
			reading: &api.DataReading{
				DataGatherer: "ark/discovery",
				Data:         nil,
			},
			expectError: `programmer mistake: the DataReading must have data type *api.DiscoveryData. This DataReading (ark/discovery) has data type <nil>`,
		},
		{
			name: "wrong data type",
			reading: &api.DataReading{
				DataGatherer: "ark/discovery",
				Data:         &api.DynamicData{},
			},
			expectError: `programmer mistake: the DataReading must have data type *api.DiscoveryData. This DataReading (ark/discovery) has data type *api.DynamicData`,
		},
		{
			name: "nil server version",
			reading: &api.DataReading{
				DataGatherer: "ark/discovery",
				Data:         &api.DiscoveryData{},
			},
			expectedSnapshot: dataupload.Snapshot{},
		},
		{
			name: "happy path",
			reading: &api.DataReading{
				DataGatherer: "ark/discovery",
				Data: &api.DiscoveryData{
					ClusterID: "success-cluster-id",
					ServerVersion: &version.Info{
						GitVersion: "v1.21.0",
					},
				},
			},
			expectedSnapshot: dataupload.Snapshot{
				ClusterID:  "success-cluster-id",
				K8SVersion: "v1.21.0",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var snapshot dataupload.Snapshot
			err := extractClusterIDAndServerVersionFromReading(test.reading, &snapshot)
			if test.expectError != "" {
				assert.EqualError(t, err, test.expectError)
				assert.Equal(t, dataupload.Snapshot{}, snapshot)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedSnapshot, snapshot)
		})
	}
}

// TestExtractResourceListFromReading tests the extractResourceListFromReading function.
func TestExtractResourceListFromReading(t *testing.T) {
	type testCase struct {
		name             string
		reading          *api.DataReading
		expectedNumItems int
		expectError      string
	}
	tests := []testCase{
		{
			name:        "nil reading",
			expectError: `programmer mistake: the DataReading must not be nil`,
		},
		{
			name: "nil data",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data:         nil,
			},
			expectError: `programmer mistake: the DataReading must have data type *api.DynamicData. ` +
				`This DataReading (ark/namespaces) has data type <nil>`,
		},
		{
			name: "wrong data type",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data:         &api.DiscoveryData{},
			},
			expectError: `programmer mistake: the DataReading must have data type *api.DynamicData. ` +
				`This DataReading (ark/namespaces) has data type *api.DiscoveryData`,
		},
		{
			name: "nil items",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data:         &api.DynamicData{},
			},
			expectedNumItems: 0,
		},
		{
			name: "empty items",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data: &api.DynamicData{
					Items: []*api.GatheredResource{},
				},
			},
			expectedNumItems: 0,
		},
		{
			name: "wrong item resource type",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data: &api.DynamicData{
					Items: []*api.GatheredResource{
						{
							Resource: &api.DiscoveryData{},
						},
					},
				},
			},
			expectError: `programmer mistake: the DynamicData items must have Resource type runtime.Object. ` +
				`This item (0) has Resource type *api.DiscoveryData`,
		},
		{
			name: "happy path",
			reading: &api.DataReading{
				DataGatherer: "ark/namespaces",
				Data: &api.DynamicData{
					Items: []*api.GatheredResource{
						{
							Resource: &unstructured.Unstructured{
								Object: map[string]any{
									"kind": "Namespace",
									"metadata": map[string]any{
										"name": "default",
										"uid":  "uid-default",
									},
								},
							},
						},
						{
							Resource: &unstructured.Unstructured{
								Object: map[string]any{
									"kind": "Namespace",
									"metadata": map[string]any{
										"name": "kube-system",
										"uid":  "uid-kube-system",
									},
								},
							},
						},
						// Deleted resource should be ignored
						{
							DeletedAt: api.Time{Time: time.Now()},
							Resource: &unstructured.Unstructured{
								Object: map[string]any{
									"kind": "Namespace",
									"metadata": map[string]any{
										"name": "kube-system",
										"uid":  "uid-kube-system",
									},
								},
							},
						},
					},
				},
			},
			expectedNumItems: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var resources []runtime.Object
			err := extractResourceListFromReading(test.reading, &resources)
			if test.expectError != "" {
				assert.EqualError(t, err, test.expectError)
				assert.Nil(t, resources)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resources)
			assert.Len(t, resources, test.expectedNumItems)
		})
	}
}

// TestConvertDataReadings_ConfigMaps tests that configmaps are correctly converted.
func TestConvertDataReadings_ConfigMaps(t *testing.T) {
	extractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": extractClusterIDAndServerVersionFromReading,
		"ark/configmaps": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.ConfigMaps)
		},
	}

	readings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "test-cluster-id",
				ServerVersion: &version.Info{
					GitVersion: "v1.21.0",
				},
			},
		},
		{
			DataGatherer: "ark/configmaps",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "conjur-connect",
									"namespace": "conjur",
									"labels": map[string]any{
										"conjur.org/name": "conjur-connect-configmap",
									},
								},
								"data": map[string]any{
									"config.yaml": "some-config-data",
								},
							},
						},
					},
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "another-configmap",
									"namespace": "default",
									"labels": map[string]any{
										"conjur.org/name": "conjur-connect-configmap",
									},
								},
								"data": map[string]any{
									"setting": "value",
								},
							},
						},
					},
					// Deleted configmap should be ignored
					{
						DeletedAt: api.Time{Time: time.Now()},
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "deleted-configmap",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
		},
	}

	var snapshot dataupload.Snapshot
	err := convertDataReadings(extractorFunctions, readings, &snapshot)
	require.NoError(t, err)

	// Verify the snapshot contains the expected data
	assert.Equal(t, "test-cluster-id", snapshot.ClusterID)
	assert.Equal(t, "v1.21.0", snapshot.K8SVersion)
	require.Len(t, snapshot.ConfigMaps, 2, "should have 2 configmaps (deleted one should be excluded)")

	// Verify the first configmap
	cm1, ok := snapshot.ConfigMaps[0].(*unstructured.Unstructured)
	require.True(t, ok, "configmap should be unstructured")
	assert.Equal(t, "ConfigMap", cm1.GetKind())
	assert.Equal(t, "conjur-connect", cm1.GetName())
	assert.Equal(t, "conjur", cm1.GetNamespace())

	// Verify the second configmap
	cm2, ok := snapshot.ConfigMaps[1].(*unstructured.Unstructured)
	require.True(t, ok, "configmap should be unstructured")
	assert.Equal(t, "ConfigMap", cm2.GetKind())
	assert.Equal(t, "another-configmap", cm2.GetName())
	assert.Equal(t, "default", cm2.GetNamespace())
}

// TestConvertDataReadings_ServiceAccounts tests that serviceaccounts are correctly converted.
func TestConvertDataReadings_ServiceAccounts(t *testing.T) {
	extractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": extractClusterIDAndServerVersionFromReading,
		"ark/serviceaccounts": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.ServiceAccounts)
		},
	}

	readings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "test-cluster-id",
				ServerVersion: &version.Info{
					GitVersion: "v1.22.0",
				},
			},
		},
		{
			DataGatherer: "ark/serviceaccounts",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ServiceAccount",
								"metadata": map[string]any{
									"name":      "default",
									"namespace": "default",
								},
							},
						},
					},
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ServiceAccount",
								"metadata": map[string]any{
									"name":      "app-sa",
									"namespace": "production",
									"labels": map[string]any{
										"app": "myapp",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var snapshot dataupload.Snapshot
	err := convertDataReadings(extractorFunctions, readings, &snapshot)
	require.NoError(t, err)

	assert.Equal(t, "test-cluster-id", snapshot.ClusterID)
	assert.Equal(t, "v1.22.0", snapshot.K8SVersion)
	require.Len(t, snapshot.ServiceAccounts, 2)

	sa1, ok := snapshot.ServiceAccounts[0].(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, "ServiceAccount", sa1.GetKind())
	assert.Equal(t, "default", sa1.GetName())
}

// TestConvertDataReadings_Roles tests that roles are correctly converted.
func TestConvertDataReadings_Roles(t *testing.T) {
	extractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": extractClusterIDAndServerVersionFromReading,
		"ark/roles": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.Roles)
		},
	}

	readings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "rbac-cluster",
				ServerVersion: &version.Info{
					GitVersion: "v1.23.0",
				},
			},
		},
		{
			DataGatherer: "ark/roles",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "rbac.authorization.k8s.io/v1",
								"kind":       "Role",
								"metadata": map[string]any{
									"name":      "pod-reader",
									"namespace": "default",
									"labels": map[string]any{
										"rbac.authorization.k8s.io/aggregate-to-view": "true",
									},
								},
								"rules": []any{
									map[string]any{
										"apiGroups": []any{""},
										"resources": []any{"pods"},
										"verbs":     []any{"get", "list"},
									},
								},
							},
						},
					},
					// Deleted role should be excluded
					{
						DeletedAt: api.Time{Time: time.Now()},
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "rbac.authorization.k8s.io/v1",
								"kind":       "Role",
								"metadata": map[string]any{
									"name":      "deleted-role",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
		},
	}

	var snapshot dataupload.Snapshot
	err := convertDataReadings(extractorFunctions, readings, &snapshot)
	require.NoError(t, err)

	assert.Equal(t, "rbac-cluster", snapshot.ClusterID)
	require.Len(t, snapshot.Roles, 1, "deleted role should be excluded")

	role, ok := snapshot.Roles[0].(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, "Role", role.GetKind())
	assert.Equal(t, "pod-reader", role.GetName())
}

// TestConvertDataReadings_MultipleResources tests conversion with multiple resource types.
func TestConvertDataReadings_MultipleResources(t *testing.T) {
	extractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": extractClusterIDAndServerVersionFromReading,
		"ark/configmaps": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.ConfigMaps)
		},
		"ark/serviceaccounts": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.ServiceAccounts)
		},
		"ark/deployments": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.Deployments)
		},
	}

	readings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "multi-resource-cluster",
				ServerVersion: &version.Info{
					GitVersion: "v1.24.0",
				},
			},
		},
		{
			DataGatherer: "ark/configmaps",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "app-config",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
		},
		{
			DataGatherer: "ark/serviceaccounts",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ServiceAccount",
								"metadata": map[string]any{
									"name":      "app-sa",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
		},
		{
			DataGatherer: "ark/deployments",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]any{
									"name":      "web-app",
									"namespace": "default",
								},
							},
						},
					},
				},
			},
		},
	}

	var snapshot dataupload.Snapshot
	err := convertDataReadings(extractorFunctions, readings, &snapshot)
	require.NoError(t, err)

	// Verify all resources are present
	assert.Equal(t, "multi-resource-cluster", snapshot.ClusterID)
	assert.Equal(t, "v1.24.0", snapshot.K8SVersion)
	require.Len(t, snapshot.ConfigMaps, 1)
	require.Len(t, snapshot.ServiceAccounts, 1)
	require.Len(t, snapshot.Deployments, 1)

	// Verify each resource type
	cm, ok := snapshot.ConfigMaps[0].(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, "app-config", cm.GetName())

	sa, ok := snapshot.ServiceAccounts[0].(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, "app-sa", sa.GetName())

	deploy, ok := snapshot.Deployments[0].(*unstructured.Unstructured)
	require.True(t, ok)
	assert.Equal(t, "web-app", deploy.GetName())
}

// TestConvertDataReadings tests the convertDataReadings function.
func TestConvertDataReadings(t *testing.T) {
	simpleExtractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": extractClusterIDAndServerVersionFromReading,
		"ark/secrets": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.Secrets)
		},
	}
	simpleReadings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
				ClusterID: "success-cluster-id",
				ServerVersion: &version.Info{
					GitVersion: "v1.21.0",
				},
			},
		},
		{
			DataGatherer: "ark/secrets",
			Data: &api.DynamicData{
				Items: []*api.GatheredResource{
					{
						Resource: &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app-1",
								Namespace: "team-1",
							},
						},
					},
					// Deleted secret should be ignored
					{
						DeletedAt: api.Time{Time: time.Now()},
						Resource: &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "deleted-1",
								Namespace: "team-1",
							},
						},
					},
				},
			},
		},
	}

	type testCase struct {
		name               string
		extractorFunctions map[string]func(*api.DataReading, *dataupload.Snapshot) error
		readings           []*api.DataReading
		expectedSnapshot   dataupload.Snapshot
		expectError        string
	}
	tests := []testCase{
		{
			name:               "no extractor functions",
			readings:           simpleReadings,
			extractorFunctions: map[string]func(*api.DataReading, *dataupload.Snapshot) error{},
			expectError:        `unexpected data gatherers, missing: [], unhandled: [ark/discovery ark/secrets]`,
		},
		{
			name:               "nil extractor functions",
			readings:           simpleReadings,
			extractorFunctions: nil,
			expectError:        `unexpected data gatherers, missing: [], unhandled: [ark/discovery ark/secrets]`,
		},
		{
			name:               "empty readings",
			extractorFunctions: simpleExtractorFunctions,
			readings:           []*api.DataReading{},
			expectError:        `unexpected data gatherers, missing: [ark/discovery ark/secrets], unhandled: []`,
		},
		{
			name:               "nil readings",
			extractorFunctions: simpleExtractorFunctions,
			readings:           nil,
			expectError:        `unexpected data gatherers, missing: [ark/discovery ark/secrets], unhandled: []`,
		},
		{
			name:               "extractor function error",
			extractorFunctions: simpleExtractorFunctions,
			readings: []*api.DataReading{
				{
					DataGatherer: "ark/discovery",
					Data:         &api.DynamicData{},
				},
			},
			expectError: `while extracting data reading ark/discovery: programmer mistake: the DataReading must have data type *api.DiscoveryData. This DataReading (ark/discovery) has data type *api.DynamicData`,
		},
		{
			name:               "happy path",
			extractorFunctions: simpleExtractorFunctions,
			readings:           simpleReadings,
			expectedSnapshot: dataupload.Snapshot{
				ClusterID:  "success-cluster-id",
				K8SVersion: "v1.21.0",
				Secrets: []runtime.Object{
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "app-1",
							Namespace: "team-1",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var snapshot dataupload.Snapshot
			err := convertDataReadings(test.extractorFunctions, test.readings, &snapshot)
			if test.expectError != "" {
				assert.EqualError(t, err, test.expectError)
				assert.Equal(t, dataupload.Snapshot{}, snapshot)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedSnapshot, snapshot)
		})
	}

}

// TestMinimizeSnapshot tests the minimizeSnapshot function.
// It creates a snapshot with various secrets and service accounts, runs
// minimizeSnapshot on it, and checks that the resulting snapshot only contains
// the expected secrets and service accounts.
func TestMinimizeSnapshot(t *testing.T) {
	secretWithClientCert := newTLSSecret("tls-secret-with-client", sampleCertificateChain(t, x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth))
	secretWithoutClientCert := newTLSSecret("tls-secret-without-client", sampleCertificateChain(t, x509.ExtKeyUsageServerAuth))
	opaqueSecret := newOpaqueSecret("opaque-secret")
	serviceAccount := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]any{
				"name":      "my-service-account",
				"namespace": "default",
			},
		},
	}

	type testCase struct {
		name             string
		inputSnapshot    dataupload.Snapshot
		expectedSnapshot dataupload.Snapshot
	}
	tests := []testCase{
		{
			name: "empty snapshot",
			inputSnapshot: dataupload.Snapshot{
				AgentVersion:    "v1.0.0",
				ClusterID:       "cluster-1",
				K8SVersion:      "v1.21.0",
				Secrets:         []runtime.Object{},
				ServiceAccounts: []runtime.Object{},
				Roles:           []runtime.Object{},
			},
			expectedSnapshot: dataupload.Snapshot{
				AgentVersion:    "v1.0.0",
				ClusterID:       "cluster-1",
				K8SVersion:      "v1.21.0",
				Secrets:         []runtime.Object{},
				ServiceAccounts: []runtime.Object{},
				Roles:           []runtime.Object{},
			},
		},
		{
			name: "snapshot with various secrets and service accounts",
			inputSnapshot: dataupload.Snapshot{
				AgentVersion: "v1.0.0",
				ClusterID:    "cluster-1",
				K8SVersion:   "v1.21.0",
				Secrets: []runtime.Object{
					secretWithClientCert,
					secretWithoutClientCert,
					opaqueSecret,
				},
				ServiceAccounts: []runtime.Object{
					serviceAccount,
				},
				Roles: []runtime.Object{},
			},
			expectedSnapshot: dataupload.Snapshot{
				AgentVersion: "v1.0.0",
				ClusterID:    "cluster-1",
				K8SVersion:   "v1.21.0",
				Secrets: []runtime.Object{
					secretWithClientCert,
					opaqueSecret,
				},
				ServiceAccounts: []runtime.Object{
					serviceAccount,
				},
				Roles: []runtime.Object{},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := ktesting.NewLogger(t, ktesting.DefaultConfig)
			minimizeSnapshot(log, &test.inputSnapshot)
			assert.Equal(t, test.expectedSnapshot, test.inputSnapshot)
		})
	}
}

// TestIsExcludableSecret tests the isExcludableSecret function.
func TestIsExcludableSecret(t *testing.T) {
	type testCase struct {
		name    string
		secret  runtime.Object
		exclude bool
	}

	tests := []testCase{
		{
			name:    "TLS secret with client cert in tls.crt",
			secret:  newTLSSecret("tls-secret-with-client", sampleCertificateChain(t, x509.ExtKeyUsageClientAuth)),
			exclude: false,
		},
		{
			name:    "TLS secret with non-client cert in tls.crt",
			secret:  newTLSSecret("tls-secret-without-client", sampleCertificateChain(t, x509.ExtKeyUsageServerAuth)),
			exclude: true,
		},
		{
			name: "Non-unstructured",
			secret: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-unstructured-secret",
					Namespace: "default",
				},
			},
			exclude: false,
		},
		{
			name: "Non-secret",
			secret: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "cert-manager/v1",
					"kind":       "Certificate",
					"metadata": map[string]any{
						"name":      "non-secret",
						"namespace": "default",
					},
				},
			},
			exclude: false,
		},
		{
			name:    "Non-TLS secret",
			secret:  newOpaqueSecret("non-tls-secret"),
			exclude: false,
		},
		{
			name:    "TLS secret without tls.crt",
			secret:  newTLSSecret("tls-secret-with-no-cert", nil),
			exclude: true,
		},
		{
			name:    "TLS secret with empty tls.crt",
			secret:  newTLSSecret("tls-secret-with-empty-cert", ""),
			exclude: true,
		},
		{
			name:    "TLS secret with invalid base64 in tls.crt",
			secret:  newTLSSecret("tls-secret-with-invalid-cert", "invalid-base64"),
			exclude: true,
		},
		{
			name:    "TLS secret with invalid PEM in tls.crt",
			secret:  newTLSSecret("tls-secret-with-invalid-pem", base64.StdEncoding.EncodeToString([]byte("invalid-pem"))),
			exclude: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			log := ktesting.NewLogger(t, ktesting.DefaultConfig)
			excluded := isExcludableSecret(log, tc.secret)
			assert.Equal(t, tc.exclude, excluded, "case: %s", tc.name)
		})
	}
}

// newTLSSecret creates a Kubernetes TLS secret with the given name and certificate data.
// If crt is nil, the secret will not contain a "tls.crt" entry.
func newTLSSecret(name string, crt any) *unstructured.Unstructured {
	data := map[string]any{"tls.key": "dummy-key"}
	if crt != nil {
		data["tls.crt"] = crt
	}
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "default",
			},
			"type": "kubernetes.io/tls",
			"data": data,
		},
	}
}

// newOpaqueSecret creates a Kubernetes Opaque secret with the given name.
func newOpaqueSecret(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]any{
				"name":      name,
				"namespace": "default",
			},
			"type": "Opaque",
			"data": map[string]any{
				"key": "value",
			},
		},
	}
}

// sampleCertificateChain returns a PEM encoded sample certificate chain for testing purposes.
// The leaf certificate is signed by a self-signed CA certificate.
// Uses an elliptic curve key for the CA and leaf certificates for speed.
// The returned string is base64 encoded to match how TLS certificates
// are typically provided in Kubernetes secrets.
func sampleCertificateChain(t testing.TB, usages ...x509.ExtKeyUsage) string {
	t.Helper()

	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertDER,
	})

	clientPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "example.com",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: usages,
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, &caTemplate, &clientPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	clientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertDER,
	})

	return base64.StdEncoding.EncodeToString(append(clientCertPEM, caCertPEM...))
}
