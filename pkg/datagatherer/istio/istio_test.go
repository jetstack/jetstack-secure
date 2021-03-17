package istio

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"gopkg.in/yaml.v2"
)

const tempFilePrefix = "preflight-test-istio-datagatherer"

// TestFetch runs a full test of the Istio data gatherer; running a fake Kubernetes API, using dynamic data gatherers to
// fetch resources, running fetched resources through Istio analysis, and checking that the analysis messaged generated
// are what was expected.
func TestFetch(t *testing.T) {
	// Local server to handle requests made by Kubernetes dynamic data gatherer. Injecting a fake client into the
	// Kubernetes data gatherer used inside the Istio data gatherer is not supported, so this instead uses an httptest
	// LocalServer to mock requests from a real dynamic client.
	localServer := createLocalTestServer(t)

	// Parse the URL of the server to generate the kubeconfig file.
	parsedURL, err := url.Parse(localServer.URL)
	if err != nil {
		t.Fatalf("failed to parse test server url %s", localServer.URL)
	}

	// Ensure there is a valid kubeconfig in a temporary file for the dynamic data gatherer.
	kubeConfigPath, err := createKubeConfigWithServer(parsedURL.String())
	if err != nil {
		t.Fatalf("failed to create temp kubeconfig: %s", err)
	}
	defer os.Remove(kubeConfigPath)

	ctx := context.Background()
	// Create the Config for the test.
	config := Config{}
	err = yaml.Unmarshal([]byte(fmt.Sprintf(configString, kubeConfigPath)), &config)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}
	dataGatherer, err := config.NewDataGatherer(ctx)
	if err != nil {
		t.Fatalf("unexpected error creating data gatherer: %+v", err)
	}

	istioDg := dataGatherer.(*DataGatherer)
	istioDg.Run(ctx.Done())
	err = istioDg.WaitForCacheSync(ctx.Done())
	if err != nil {
		t.Fatalf("unexpected client error: %+v", err)
	}

	// Fetch analysis result from the data gatherer.
	rawAnalysisResult, err := istioDg.Fetch()
	if err != nil {
		t.Fatalf("unexpected error fetching results: %+v", err)
	}

	// Unpack the analysis result to find the code of the first message for checking.
	var analysisResult map[string]interface{}
	if err := json.Unmarshal([]byte(rawAnalysisResult.(string)), &analysisResult); err != nil {
		t.Fatalf("unexpected error unmarshalling analysis result: %+v", err)
	}
	analysisMessages, ok := analysisResult["Messages"].([]interface{})
	if !ok {
		t.Fatalf("%+v", analysisResult["Messages"])
	}
	analysisMessage, ok := analysisMessages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("%+v", analysisMessages[0])
	}
	analysisMessageCode, ok := analysisMessage["code"].(string)
	if !ok {
		t.Fatalf("%+v", analysisMessage["code"])
	}

	// With the test configuration and fake resources provided there should be only one message to warn about a missing
	// Istio annotation on the default Namespace, with code IST0102.
	if analysisMessageCode != "IST0102" {
		t.Fatalf("unexpected analysis result messages: %+v", analysisMessageCode)
	}
}

var configString = `
kubeconfig: %s
resources:
- group:    ""
  version:  "v1"
  resource: "namespaces"
`

// createKubeConfigWithServer creates a kubeconfig file on disk with a reference to the local server.
func createKubeConfigWithServer(server string) (string, error) {
	content := fmt.Sprintf(kubeConfigString, server)
	tempFile, err := ioutil.TempFile("", tempFilePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create a tmpfile for kubeconfig")
	}

	if _, err := tempFile.Write([]byte(content)); err != nil {
		return "", fmt.Errorf("failed to write to tmp kubeconfig file")
	}
	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close tmp kubeconfig file after writing")
	}

	return tempFile.Name(), nil
}

var kubeConfigString = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: example
contexts:
- context:
    cluster: example
    namespace: default
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    username: test
    password: test
`

// createLocalTestServer creates a local test server to respond to Kubernetes API requests from the dynamic data
// gatherer.
func createLocalTestServer(t *testing.T) *httptest.Server {
	var localServer *httptest.Server
	localServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var responseContent []byte

		switch r.URL.Path {
		case "/api/v1/namespaces":
			responseContent = []byte(testNamespaces)
		default:
			t.Fatalf("Unexpected URL was called: %s", r.URL.Path)
		}

		w.Write(responseContent)
	}))

	return localServer
}

var testNamespaces = `
{
  "apiVersion": "v1",
  "items": [
    {
      "apiVersion": "v1",
      "kind": "Namespace",
      "metadata": {
        "name": "default"
      }
    }
  ],
  "kind": "List",
  "metadata": {
    "resourceVersion": "",
    "selfLink": ""
  }
}
`
