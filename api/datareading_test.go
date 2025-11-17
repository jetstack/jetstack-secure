package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJSONGatheredResourceDropsEmptyTime(t *testing.T) {
	var resource GatheredResource
	bytes, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal %s", err)
	}

	expected := `{"resource":null}`

	if string(bytes) != expected {
		t.Fatalf("unexpected json \ngot  %s\nwant %s", string(bytes), expected)
	}
}

func TestJSONGatheredResourceSetsTimeWhenPresent(t *testing.T) {
	var resource GatheredResource
	resource.DeletedAt = Time{time.Date(2021, 3, 29, 0, 0, 0, 0, time.UTC)}
	bytes, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal %s", err)
	}

	expected := `{"resource":null,"deleted_at":"2021-03-29T00:00:00Z"}`

	if string(bytes) != expected {
		t.Fatalf("unexpected json \ngot  %s\nwant %s", string(bytes), expected)
	}
}

// TestDataReading_UnmarshalJSON tests the UnmarshalJSON method of DataReading
// with various scenarios including valid and invalid JSON inputs.
func TestDataReading_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantDataType any
		expectError  string
	}{
		{
			name: "DiscoveryData type",
			input: `{
				"cluster_id": "61b2db64-fd70-49a6-a257-08397b9b4bae",
				"data-gatherer": "discovery",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": {
                    "cluster_id": "60868ebf-6e47-4184-9bc0-20bb6824e210",
					"server_version": {
                        "major": "1",
                        "minor": "20",
                        "gitVersion": "v1.20.0"
                    }
                },
				"schema_version": "v1"
			}`,
			wantDataType: &DiscoveryData{},
		},
		{
			name: "DynamicData type",
			input: `{
				"cluster_id": "69050b54-c61a-4384-95c3-35f890377a67",
				"data-gatherer": "dynamic",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": {"items": []},
				"schema_version": "v1"
			}`,
			wantDataType: &DynamicData{},
		},
		{
			name:        "Invalid JSON",
			input:       `not a json`,
			expectError: "failed to parse DataReading: invalid character 'o' in literal null (expecting 'u')",
		},
		{
			name: "Missing data field",
			input: `{
				"cluster_id": "cc5a0429-8dc4-42c8-8e3a-eece9bca15c3",
				"data-gatherer": "missing-data-field",
				"timestamp": "2024-06-01T12:00:00Z",
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "missing-data-field": empty data`,
		},
		{
			name: "Mismatched data type",
			input: `{
				"cluster_id": "c272b13e-b19e-4782-833f-d55a305f3c9e",
				"data-gatherer": "unknown-data-type",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": "this should be an object",
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "unknown-data-type": unknown type`,
		},
		{
			name: "Empty data field",
			input: `{
				"cluster_id": "07909675-113f-4b59-ba5e-529571a191e6",
				"data-gatherer": "empty-data",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": {},
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "empty-data": empty data`,
		},
		{
			name: "Additional field",
			input: `{
				"cluster_id": "11df7332-4b32-4f5a-903b-0cbbef381850",
				"data-gatherer": "additional-field",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": {
					"cluster_id": "60868ebf-6e47-4184-9bc0-20bb6824e210"
				},
				"extra_field": "should cause error",
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading: json: unknown field "extra_field"`,
		},
		{
			name: "Additional data field",
			input: `{
				"cluster_id": "ca44c338-987e-4d57-8320-63f538db4292",
				"data-gatherer": "additional-data-field",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": {
					"cluster_id": "60868ebf-6e47-4184-9bc0-20bb6824e210",
					"server_version": {
						"major": "1",
						"minor": "20",
						"gitVersion": "v1.20.0"
  					},
					"extra_field": "should cause error"
				},
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "additional-data-field": unknown type`,
		},
		{
			name:        "Empty JSON object",
			input:       `{}`,
			expectError: `failed to parse DataReading.Data for gatherer "": empty data`,
		},
		{
			name: "Null data field",
			input: `{
				"cluster_id": "36281cb3-7f3a-4efa-9879-7c988a9715b0",
				"data-gatherer": "null-data",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": null,
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "null-data": empty data`,
		},
		{
			name: "Empty string data field",
			input: `{
				"cluster_id": "7b7aa8ee-58ac-4818-9b29-c0a76296ea1d",
				"data-gatherer": "empty-string-data",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": "",
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "empty-string-data": unknown type`,
		},
		{
			name: "Array instead of object in data field",
			input: `{
				"cluster_id": "94d7757f-d084-4ccb-963b-f60fece0df2d",
				"data-gatherer": "array-data",
				"timestamp": "2024-06-01T12:00:00Z",
				"data": [],
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading.Data for gatherer "array-data": unknown type`,
		},
		{
			name: "Incorrect timestamp format",
			input: `{
				"cluster_id": "d58f298d-b8c1-4d99-aa85-c27d9aec6f97",
				"data-gatherer": "bad-timestamp",
				"timestamp": "not-a-timestamp",
				"data": {
					"items": []
				},
				"schema_version": "v1"
			}`,
			expectError: `failed to parse DataReading: parsing time "not-a-timestamp" as "2006-01-02T15:04:05Z07:00": cannot parse "not-a-timestamp" as "2006"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dr DataReading
			err := dr.UnmarshalJSON([]byte(tt.input))
			if tt.expectError != "" {
				assert.EqualError(t, err, tt.expectError)
				return
			}
			assert.NoError(t, err)
			assert.IsType(t, tt.wantDataType, dr.Data)
		})
	}
}
