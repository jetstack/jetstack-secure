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
		name         string
		path         string
		readings     []*api.DataReading
		expectedJSON string
		// expectedError asserts the whole error message.
		expectedError string
		// expectedErrorContains asserts fragments instead, for messages that
		// embed a standard library type name. Go 1.27 re-implemented
		// encoding/json on top of encoding/json/v2, so a MarshalJSON error
		// against a json.RawMessage now names *jsontext.Value.
		expectedErrorContains []string
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
			expectedErrorContains: []string{
				"failed to marshal JSON",
				"invalid character 'x' looking for beginning of value",
			},
			expectedJSON: "[]",
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
			if len(tc.expectedErrorContains) > 0 {
				for _, want := range tc.expectedErrorContains {
					assert.ErrorContains(t, err, want)
				}
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
