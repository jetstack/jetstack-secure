package k8s

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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
				return fmt.Errorf("failed to select data at JSONPointer: %s", v)
			}
			pathComponents := strings.Split(v, "/")
			filteredObject.Set(gObject.Data(), pathComponents[1:]...)
		} else {
			filteredObject.SetP(jsonParsed.Path(v).Data(), v)
		}
	}

	// load the filtered JSON back into the resource
	err = json.Unmarshal(filteredObject.Bytes(), resource)
	if err != nil {
		return fmt.Errorf("failed to update resource: %s", err)
	}

	return nil
}
