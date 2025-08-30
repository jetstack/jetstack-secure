package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

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

func TestConvertDataReadings(t *testing.T) {
	simpleExtractorFunctions := map[string]func(*api.DataReading, *dataupload.Snapshot) error{
		"ark/discovery": func(reading *api.DataReading, snapshot *dataupload.Snapshot) error {
			return extractServerVersionFromReading(reading, &snapshot.K8SVersion)
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
			expectError:        `unexpected data gatherers, missing: [], unhandled: [ark/discovery]`,
		},
		{
			name:               "nil extractor functions",
			readings:           simpleReadings,
			extractorFunctions: nil,
			expectError:        `unexpected data gatherers, missing: [], unhandled: [ark/discovery]`,
		},
		{
			name:               "empty readings",
			extractorFunctions: simpleExtractorFunctions,
			readings:           []*api.DataReading{},
			expectError:        `unexpected data gatherers, missing: [ark/discovery], unhandled: []`,
		},
		{
			name:               "nil readings",
			extractorFunctions: simpleExtractorFunctions,
			readings:           nil,
			expectError:        `unexpected data gatherers, missing: [ark/discovery], unhandled: []`,
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

func TestConvertDataReadingsToCyberarkSnapshot_Golden(t *testing.T) {
	dataReadings := parseDataReadings(t, readGZIP(t, "testdata/example-1/datareadings.json.gz"))
	var snapshot dataupload.Snapshot
	err := convertDataReadings(defaultExtractorFunctions, dataReadings, &snapshot)
	require.NoError(t, err)

	actualSnapshotBytes, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)

	goldenFilePath := "testdata/example-1/snapshot.json.gz"
	if _, update := os.LookupEnv("UPDATE_GOLDEN_FILES"); update {
		writeGZIP(t, goldenFilePath, actualSnapshotBytes)
	} else {
		expectedSnapshotBytes := readGZIP(t, goldenFilePath)
		assert.JSONEq(t, string(expectedSnapshotBytes), string(actualSnapshotBytes))
	}
}

// parseDataReadings decodes JSON encoded datareadings.
// It attempts to decode the data of each reading into a concrete type.
// It tries to decode the data as DynamicData and DiscoveryData and then gives
// up with a test failure.
// This function is useful for reading sample datareadings from disk for use in
// CyberArk dataupload client tests, which require the datareadings data to be runtime.Object.
func parseDataReadings(t *testing.T, data []byte) []*api.DataReading {
	var dataReadings []*api.DataReading

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&dataReadings)
	require.NoError(t, err)

	for _, reading := range dataReadings {
		dataBytes, err := json.Marshal(reading.Data)
		require.NoError(t, err)
		in := bytes.NewReader(dataBytes)
		d := json.NewDecoder(in)
		d.DisallowUnknownFields()

		var dynamicGatherData api.DynamicData
		if err := d.Decode(&dynamicGatherData); err == nil {
			reading.Data = &dynamicGatherData
			continue
		}

		_, err = in.Seek(0, 0)
		require.NoError(t, err)

		var discoveryData api.DiscoveryData
		if err = d.Decode(&discoveryData); err == nil {
			reading.Data = &discoveryData
			continue
		}

		require.Failf(t, "failed to parse reading", "reading: %#v", reading)
	}
	return dataReadings
}

// readGZIP Reads the gzip file at path, and returns the decompressed bytes
func readGZIP(t *testing.T, path string) []byte {
	f, err := os.Open(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { require.NoError(t, gzr.Close()) }()
	bytes, err := io.ReadAll(gzr)
	require.NoError(t, err)
	return bytes
}

// writeGZIP writes gzips the data and writes it to path.
func writeGZIP(t *testing.T, path string, data []byte) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	require.NoError(t, err)
	gzw := gzip.NewWriter(tmp)
	_, err = gzw.Write(data)
	require.NoError(t, errors.Join(
		err,
		gzw.Flush(),
		gzw.Close(),
		tmp.Close(),
	))
	err = os.Rename(tmp.Name(), path)
	require.NoError(t, err)
}
