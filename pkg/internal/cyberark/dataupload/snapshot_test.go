package dataupload

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/pkg/internal/cyberark/testutil"
)

func TestConvertDataReadingsToCyberarkSnapshot(t *testing.T) {
	dataReadings := testutil.ParseDataReadings(t, testutil.ReadGZIP(t, "testdata/example-1/datareadings.json.gz"))
	snapshot, err := convertDataReadingsToCyberarkSnapshot(dataReadings)
	require.NoError(t, err)

	actualSnapshotBytes, err := json.MarshalIndent(snapshot, "", "  ")
	require.NoError(t, err)

	goldenFilePath := "testdata/example-1/snapshot.json.gz"
	if _, update := os.LookupEnv("UPDATE_GOLDEN_FILES"); update {
		testutil.WriteGZIP(t, goldenFilePath, actualSnapshotBytes)
	} else {
		expectedSnapshotBytes := testutil.ReadGZIP(t, goldenFilePath)
		assert.JSONEq(t, string(expectedSnapshotBytes), string(actualSnapshotBytes))
	}
}
