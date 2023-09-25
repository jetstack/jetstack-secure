package k8s

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SecretSelectedFields is the list of fields sent from Secret objects to the
// backend
var SecretSelectedFields = []string{
	"kind",
	"apiVersion",
	"metadata.annotations",
	"metadata.name",
	"metadata.namespace",
	"metadata.ownerReferences",
	"metadata.selfLink",
	"metadata.uid",
	"type",
	"/data/tls.crt",
	"/data/ca.crt",
}

// RedactFields are removed from all objects
var RedactFields = []string{
	"metadata.managedFields",
	"/metadata/annotations/kubectl.kubernetes.io~1last-applied-configuration",
}

// Select removes all but the supplied fields from the resource
func Select(fields []string, resource *unstructured.Unstructured) error {
	// convert the object to JSON for field filtering
	asJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal json for resource: %s", err)
	}

	// parse the JSON for processing in gabs
	jsonParsed, err := gabs.ParseJSON(asJSON)
	if err != nil {
		return fmt.Errorf("failed to parse generated json for resource: %s", err)
	}

	// craft a new object containing only selected fields
	filteredObject := gabs.New()
	for _, v := range fields {
		// also support JSONPointers for keys containing '.' chars
		if strings.HasPrefix(v, "/") {
			gObject, err := jsonParsed.JSONPointer(v)
			if err != nil {
				// fail to select field if missing, just continue
				continue
			}
			pathComponents, err := gabs.JSONPointerToSlice(v)
			if err != nil {
				return fmt.Errorf("invalid JSONPointer: %s", v)
			}
			filteredObject.Set(gObject.Data(), pathComponents...)
		} else {
			if jsonParsed.ExistsP(v) {
				filteredObject.SetP(jsonParsed.Path(v).Data(), v)
			}
		}
	}

	// load the filtered JSON back into the resource
	err = json.Unmarshal(filteredObject.Bytes(), resource)
	if err != nil {
		return fmt.Errorf("failed to update resource: %s", err)
	}

	return nil
}

// Redact removes the supplied fields from the resource
func Redact(fields []string, resource *unstructured.Unstructured) error {
	// convert the object to JSON for field filtering
	asJSON, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal json for resource: %s", err)
	}

	// parse the JSON for processing in gabs
	jsonParsed, err := gabs.ParseJSON(asJSON)
	if err != nil {
		return fmt.Errorf("failed to parse generated json for resource: %s", err)
	}

	// craft a new object excluding redacted fields
	for _, v := range fields {
		// also support JSONPointers for keys containing '.' chars
		if strings.HasPrefix(v, "/") {
			pathComponents, err := gabs.JSONPointerToSlice(v)
			if err != nil {
				return fmt.Errorf("invalid JSONPointer: %s", v)
			}
			if jsonParsed.Exists(pathComponents...) {
				jsonParsed.Delete(pathComponents...)
			}
		} else {
			if jsonParsed.ExistsP(v) {
				jsonParsed.DeleteP(v)
			}
		}
	}

	// load the filtered JSON back into the resource
	err = json.Unmarshal(jsonParsed.Bytes(), resource)
	if err != nil {
		return fmt.Errorf("failed to update resource: %s", err)
	}

	return nil
}
