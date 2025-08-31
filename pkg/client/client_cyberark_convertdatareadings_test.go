package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

// TestExtractServerVersionFromReading tests the extractServerVersionFromReading function.
func TestExtractServerVersionFromReading(t *testing.T) {
	type testCase struct {
		name            string
		reading         *api.DataReading
		expectedVersion string
		expectError     string
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
			expectedVersion: "",
		},
		{
			name: "happy path",
			reading: &api.DataReading{
				DataGatherer: "ark/discovery",
				Data: &api.DiscoveryData{
					ServerVersion: &version.Info{
						GitVersion: "v1.21.0",
					},
				},
			},
			expectedVersion: "v1.21.0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var k8sVersion string
			err := extractServerVersionFromReading(test.reading, &k8sVersion)
			if test.expectError != "" {
				assert.EqualError(t, err, test.expectError)
				assert.Equal(t, "", k8sVersion)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.expectedVersion, k8sVersion)
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
								Object: map[string]interface{}{
									"kind": "Namespace",
									"metadata": map[string]interface{}{
										"name": "default",
										"uid":  "uid-default",
									},
								},
							},
						},
						{
							Resource: &unstructured.Unstructured{
								Object: map[string]interface{}{
									"kind": "Namespace",
									"metadata": map[string]interface{}{
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

// TestConvertDataReadings tests the convertDataReadings function.
func TestConvertDataReadings(t *testing.T) {
	simpleExtractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractServerVersionFromReading(reading, &snapshot.K8SVersion)
		},
		"ark/secrets": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractResourceListFromReading(reading, &snapshot.Secrets)
		},
	}
	simpleReadings := []*api.DataReading{
		{
			DataGatherer: "ark/discovery",
			Data: &api.DiscoveryData{
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
