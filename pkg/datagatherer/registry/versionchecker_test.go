package versionchecker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/jetstack/preflight/pkg/datagatherer/k8s"
	vcclient "github.com/jetstack/version-checker/pkg/client"
	vcselfhosted "github.com/jetstack/version-checker/pkg/client/selfhosted"
)

func Test1(t *testing.T) {
	// create a local test server to respond to k8s and registry api requests
	var localServer *httptest.Server
	localServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var responseContent []byte

		if r.URL.Path == "/api/v1/pods" {
			// the responses from the server are self referential and the host is
			// needed to generate responses
			parsedURL, err := url.Parse(localServer.URL)
			if err != nil {
				t.Fatalf("failed to parse test server url %s", localServer.URL)
			}

			tmpl, err := template.ParseFiles("fixtures/pods.json.tmpl")
			if err != nil {
				t.Fatalf("failed to load template files: %s", err)
			}

			// generate a response that contains pods pointing to this server
			// as the registry
			var response bytes.Buffer
			err = tmpl.Execute(&response, struct{ URL *string }{&parsedURL.Host})
			if err != nil {
				t.Fatalf("failed to exe template: %s", err)
			}
			responseContent = response.Bytes()
		} else if r.URL.Path == "/v2/jetstack/example/tags/list" {
			file, err := os.Open("fixtures/tags.json")
			if err != nil {
				t.Fatalf("failed to open tags fixture: %s", err)
			}
			defer file.Close()

			responseContent, err = ioutil.ReadAll(file)
			if err != nil {
				t.Fatalf("failed to read tags fixture: %s", err)
			}
		} else if r.URL.Path == "/v2/jetstack/example/manifests/v1.0.0" {
			responseContent = []byte(`{
			  "schemaVersion": 1,
			  "name": "jetstack/example",
			  "tag": "v1.0.0"
			}`)
		} else if r.URL.Path == "/v2/jetstack/example/manifests/v1.0.1" {
			responseContent = []byte(`{
			  "schemaVersion": 1,
			  "name": "jetstack/example",
			  "tag": "v1.0.1"
			}`)
		} else {
			t.Fatalf("Unexpected URL was called: %s", r.URL.Path)
		}

		fmt.Fprint(w, string(responseContent))
	}))

	// parse the URL of the server to generate the KubeConfig file
	parsedURL, err := url.Parse(localServer.URL)
	if err != nil {
		t.Fatalf("failed to parse test server url %s", localServer.URL)
	}

	content := fmt.Sprintf(`
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
    password: test`, parsedURL)
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		t.Fatalf("failed to parse test server url %s", err)
	}

	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		log.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatal(err)
	}

	// create the config for the DataGatherer, wraps config for Dynamic client
	// and version checker
	config := Config{
		Dynamic: k8s.ConfigDynamic{
			KubeConfigPath: tmpfile.Name(),
		},
		VersionCheckerClientOptions: vcclient.Options{
			Selfhosted: map[string]*vcselfhosted.Options{
				"test": {
					Host: "http://" + parsedURL.Host,
				},
			},
		},
	}

	dg, err := config.NewDataGatherer(context.Background())
	if err != nil {
		t.Fatalf("failed create new dg %s", err)
	}

	rawResults, err := dg.Fetch()
	if err != nil {
		t.Fatalf("failed fetch data %s", err)
	}

	resultsJSON, err := json.MarshalIndent(rawResults, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal data %s", err)
	}

	expectedResultsJSON := fmt.Sprintf(`[
  {
    "Pod": {
      "kind": "Pod",
      "apiVersion": "v1",
      "metadata": {
        "name": "example-6d94489854-zpzhr",
        "namespace": "example",
        "selfLink": "/api/v1/namespaces/example/pods/example-6d94489854-zpzhr",
        "uid": "efff9dae-28ca-42c3-be70-970731c44f67",
        "resourceVersion": "32023849",
        "creationTimestamp": null,
        "labels": {
          "app": "example"
        },
        "ownerReferences": [
          {
            "apiVersion": "apps/v1",
            "kind": "ReplicaSet",
            "name": "example-6d94489854",
            "uid": "bb6c0f31-0e28-4c28-a81d-91b8d7bfed33",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ]
      },
      "spec": {
        "containers": [
          {
            "name": "example",
            "image": "%s/jetstack/example:v1.0.0",
            "command": [
              "sh",
              "-c"
            ],
            "resources": {}
          }
        ]
      },
      "status": {
        "containerStatuses": [
          {
            "name": "example",
            "state": {},
            "lastState": {},
            "ready": false,
            "restartCount": 0,
            "image": "",
            "imageID": "is set"
          }
        ]
      }
    },
    "Result": {
      "CurrentVersion": "v1.0.0",
      "LatestVersion": "v1.0.1",
      "IsLatest": false,
      "ImageURL": "%s/jetstack/example"
    }
  }
]`, parsedURL.Host, parsedURL.Host)

	if expectedResultsJSON != string(resultsJSON) {
		t.Fatalf("results json does not match: %s vs %s", resultsJSON, expectedResultsJSON)
	}
}
