/*
Copyright 2026 The cert-manager Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package baker

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

// inplaceReadValuesYAML reads the provided chart tar file and returns the values
func readValuesYAML(inputPath string) (map[string]any, error) {
	var result map[string]any
	return result, modifyValuesYAML(inputPath, "", func(m map[string]any) (map[string]any, error) {
		result = m
		return m, nil
	})
}

type modFunction func(map[string]any) (map[string]any, error)

func modifyValuesYAML(inFilePath string, outFilePath string, modFn modFunction) error {
	inReader, err := os.Open(inFilePath)
	if err != nil {
		return err
	}
	defer inReader.Close()
	outWriter := io.Discard
	if outFilePath != "" {
		outFile, err := os.Create(outFilePath)
		if err != nil {
			return err
		}
		defer outFile.Close()
		outWriter = outFile
	}
	if strings.HasSuffix(inFilePath, ".tgz") {
		if err := modifyTarStreamValuesYAML(inReader, outWriter, modFn); err != nil {
			return err
		}
	} else {
		if err := modifyStreamValuesYAML(inReader, outWriter, modFn); err != nil {
			return err
		}
	}
	return nil
}

func modifyTarStreamValuesYAML(in io.Reader, out io.Writer, modFn modFunction) error {
	inFileDecompressed, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer inFileDecompressed.Close()
	tr := tar.NewReader(inFileDecompressed)
	outFileCompressed, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	outFileCompressed.Extra = []byte("+aHR0cHM6Ly95b3V0dS5iZS96OVV6MWljandyTQo=")
	outFileCompressed.Comment = "Helm"
	defer outFileCompressed.Close()
	tw := tar.NewWriter(outFileCompressed)
	defer tw.Close()
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return err
		}
		const maxValuesYAMLSize = 2 * 1024 * 1024 // 2MB
		limitedReader := &io.LimitedReader{
			R: tr,
			N: maxValuesYAMLSize,
		}
		if strings.HasSuffix(hdr.Name, "/values.yaml") {
			var modifiedContent bytes.Buffer
			if err := modifyStreamValuesYAML(limitedReader, &modifiedContent, modFn); err != nil {
				return err
			}
			// Update header size
			hdr.Size = int64(modifiedContent.Len())
			// Write updated header and content
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := tw.Write(modifiedContent.Bytes()); err != nil {
				return err
			}
		} else {
			// Stream other files unchanged
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if _, err := io.Copy(tw, limitedReader); err != nil {
				return err
			}
		}
		if limitedReader.N <= 0 {
			return fmt.Errorf("values.yaml is larger than %v bytes", maxValuesYAMLSize)
		}
	}
	return nil
}

func modifyStreamValuesYAML(in io.Reader, out io.Writer, modFn modFunction) error {
	inputBytes, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	// Parse YAML into Go values for modification logic.
	var data map[string]any
	if err := yaml.Unmarshal(inputBytes, &data); err != nil {
		return err
	}
	originalStrings := map[string]string{}
	collectStringValues(data, nil, originalStrings)
	// Modify YAML values via the existing callback.
	data, err = modFn(data)
	if err != nil {
		return err
	}
	updatedStrings := map[string]string{}
	collectStringValues(data, nil, updatedStrings)
	// Parse YAML into an AST so we can update nodes without losing comments.
	astFile, err := parser.ParseBytes(inputBytes, parser.ParseComments)
	if err != nil {
		return err
	}
	for yamlPath, newValue := range updatedStrings {
		if originalValue, ok := originalStrings[yamlPath]; ok && originalValue == newValue {
			continue
		}
		path, err := yaml.PathString(yamlPath)
		if err != nil {
			return err
		}
		node, err := yaml.ValueToNode(newValue)
		if err != nil {
			return err
		}
		if err := path.ReplaceWithNode(astFile, node); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, astFile.String())
	return err
}

type pathSegment struct {
	key     string
	isIndex bool
}

func collectStringValues(object any, path []pathSegment, out map[string]string) {
	switch t := object.(type) {
	case map[string]any:
		for key, value := range t {
			keyPath := append(path, pathSegment{key: key})
			if stringValue, ok := value.(string); ok {
				out[pathToYAMLPath(keyPath)] = stringValue
				continue
			}
			collectStringValues(value, keyPath, out)
		}
	case map[string]string:
		for key, value := range t {
			keyPath := append(path, pathSegment{key: key})
			out[pathToYAMLPath(keyPath)] = value
		}
	case []any:
		for i, value := range t {
			keyPath := append(path, pathSegment{key: strconv.Itoa(i), isIndex: true})
			if stringValue, ok := value.(string); ok {
				out[pathToYAMLPath(keyPath)] = stringValue
				continue
			}
			collectStringValues(value, keyPath, out)
		}
	case []string:
		for i, value := range t {
			keyPath := append(path, pathSegment{key: strconv.Itoa(i), isIndex: true})
			out[pathToYAMLPath(keyPath)] = value
		}
	default:
		// ignore object
	}
}

func pathToYAMLPath(path []pathSegment) string {
	if len(path) == 0 {
		return "$"
	}
	var builder strings.Builder
	builder.WriteString("$")
	for _, segment := range path {
		if segment.isIndex {
			builder.WriteString("[")
			builder.WriteString(segment.key)
			builder.WriteString("]")
			continue
		}
		builder.WriteString(".")
		if isSimpleYAMLPathKey(segment.key) {
			builder.WriteString(segment.key)
			continue
		}
		builder.WriteString("'")
		builder.WriteString(strings.ReplaceAll(segment.key, "'", "\\'"))
		builder.WriteString("'")
	}
	return builder.String()
}

func isSimpleYAMLPathKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if i == 0 && unicode.IsDigit(r) {
			return false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
