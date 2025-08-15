package testutil

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jetstack/preflight/api"
)

// ParseDataReadings decodes JSON encoded datareadings.
// It attempts to decode the data of each reading into a concrete type.
// It tries to decode the data as DynamicData and DiscoveryData and then gives
// up with a test failure.
// This function is useful for reading sample datareadings from disk for use in
// CyberArk dataupload client tests, which require the datareadings data to have
// rich types
// TODO(wallrj): Refactor this so that it can be used with the `agent
// --input-path` feature, to enable datareadings to be read from disk and pushed
// to CyberArk.
func ParseDataReadings(t *testing.T, data []byte) []*api.DataReading {
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

// ReadGZIP Reads the gzip file at path, and returns the decompressed bytes
func ReadGZIP(t *testing.T, path string) []byte {
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

// WriteGZIP writes gzips the data and writes it to path.
func WriteGZIP(t *testing.T, path string, data []byte) {
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
