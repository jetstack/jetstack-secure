package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceWithStaticTimestamps(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "klog",
			input:    `I1018 15:20:42.861239    2386 logs_test.go:13] "Contextual Info Level 3" logger="foo" key="value"`,
			expected: `I0000 00:00:00.000000   00000 logs_test.go:000] "Contextual Info Level 3" logger="foo" key="value"`,
		},
		{
			name:     "klog without process ID and without file name",
			input:    `E1114 11:15:39.455086] Cache update failure err="not a cacheResource type: *k8s.notCachable missing metadata/uid field" operation="add"`,
			expected: `E0000 00:00:00.000000] Cache update failure err="not a cacheResource type: *k8s.notCachable missing metadata/uid field" operation="add"`,
		},
		{
			name:     "json-with-nanoseconds",
			input:    `{"ts":1729270111728.125,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}`,
			expected: `{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Warn","v":0}`,
		},
		{
			name:     "json-might-not-have-nanoseconds",
			input:    `{"ts":1729270111728,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}`,
			expected: `{"ts":0000000000000.000,"caller":"logs/logs_test.go:000","msg":"slog Info","v":0}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, ReplaceWithStaticTimestamps(test.input))
		})
	}
}
