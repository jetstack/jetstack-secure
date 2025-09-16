package client

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
)

func TestFileClient_PostDataReadingsWithOptions(t *testing.T) {
	type testCase struct {
		name          string
		path          string
		readings      []*api.DataReading
		expectedJSON  string
		expectedError string
	}
	tests := []testCase{
		{
			name:         "success",
			path:         "{tmp}/data.json",
			readings:     []*api.DataReading{},
			expectedJSON: "[]",
		},
		{
			name:         "success-overwrite",
			path:         "{tmp}/exists.json",
			readings:     []*api.DataReading{},
			expectedJSON: "[]",
		},
		{
			name: "json-marshal-error",
			path: "{tmp}/data.json",
			readings: []*api.DataReading{
				{
					Data: json.RawMessage("x"),
				},
			},
			expectedError: "failed to marshal JSON: json: error calling MarshalJSON for type json.RawMessage: invalid character 'x' looking for beginning of value",
			expectedJSON:  "[]",
		},
		{
			name:          "no-such-file-or-directory",
			path:          "{tmp}/no-such-folder/data.json",
			readings:      []*api.DataReading{},
			expectedError: "failed to write file: open {tmp}/no-such-folder/data.json: no such file or directory",
			expectedJSON:  "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log := ktesting.NewLogger(t, ktesting.DefaultConfig)
			ctx := klog.NewContext(t.Context(), log)
			tmpDir := t.TempDir()
			require.NoError(t, os.WriteFile(tmpDir+"/exists.json", []byte("existing-content"), 0644))

			path := strings.ReplaceAll(tc.path, "{tmp}", tmpDir)
			expectedError := strings.ReplaceAll(tc.expectedError, "{tmp}", tmpDir)

			c := NewFileClient(path)
			err := c.PostDataReadingsWithOptions(ctx, tc.readings, Options{})

			if expectedError != "" {
				assert.EqualError(t, err, expectedError)
				return
			}
			require.NoError(t, err)
			assert.FileExists(t, path)
			actualJSON, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.JSONEq(t, tc.expectedJSON, string(actualJSON))
		})
	}
}
