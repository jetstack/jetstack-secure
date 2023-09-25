package versionchecker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const tmpFilePrefix = "preflight-test-file"

func TestUnmarshalConfig(t *testing.T) {
	textCfg := `
k8s:
  kubeconfig: "/home/someone/.kube/config"
  resource-type:
    # not usually set by users, but here to test defaulting
    group: "g"
    version: "v"
    resource: "r"
  exclude-namespaces:
  - kube-system
  include-namespaces:
  # invalid to have in addition to exclude, but used to get config loading
  - default
registries:
- kind: acr
  params:
    username: fixtures/example_secret
    password: fixtures/example_secret
    refresh_token: fixtures/example_secret
- kind: ecr
  params:
    access_key_id: fixtures/example_secret
    secret_access_key: fixtures/example_secret
    session_token: fixtures/example_secret
- kind: gcr
  params:
    token: fixtures/example_secret
- kind: docker
  params:
    username: fixtures/example_secret
    password: fixtures/example_secret
    token: fixtures/example_secret
- kind: quay
  params:
    token: fixtures/example_secret
- kind: selfhosted
  params:
    host: fixtures/example_host
    username: fixtures/example_secret
    password: fixtures/example_secret
- kind: selfhosted
  params:
    host: fixtures/example_host_2
    bearer: fixtures/example_secret
`

	cfg := Config{}
	err := yaml.Unmarshal([]byte(textCfg), &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %+v", err)
	}

	tests := map[string]struct {
		configTarget interface{}
		expTarget    interface{}
	}{
		"KubeConfigPath": {
			configTarget: cfg.DynamicPod.KubeConfigPath,
			expTarget:    "/home/someone/.kube/config",
		},
		"GVR": {
			configTarget: cfg.DynamicPod.GroupVersionResource,
			expTarget: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "pods", // should use pods even if other gvr set
			},
		},
		"IncludeNamespaces": {
			configTarget: cfg.DynamicPod.IncludeNamespaces,
			expTarget:    []string{"default"},
		},
		"ExcludeNamespaces": {
			configTarget: cfg.DynamicPod.ExcludeNamespaces,
			expTarget:    []string{"kube-system"},
		},
		"GCR Token": {
			configTarget: cfg.VersionCheckerClientOptions.GCR.Token,
			expTarget:    "pa55w0rd",
		},
		"self hosted password": {
			configTarget: cfg.VersionCheckerClientOptions.Selfhosted["example.com"].Password,
			expTarget:    "pa55w0rd",
		},
		"self hosted bearer": {
			configTarget: cfg.VersionCheckerClientOptions.Selfhosted["example.net"].Bearer,
			expTarget:    "pa55w0rd",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if !reflect.DeepEqual(test.configTarget, test.expTarget) {
				t.Errorf("unexpected config target: got=%q; want=%q", test.configTarget, test.expTarget)
			}
		})
	}
}

// TestVersionCheckerFetch will make requests against a dummy k8s server to get
// pods, then check the found images using version checker. Version checker
// will call the same version checker to get image tag data
func TestVersionCheckerFetch(t *testing.T) {
	// server to handle requests made by version checker and k8s dynamic dg.
	//
	// since we do not support injecting a fake client as a dependency to the
	// version checker datagatherer we are instead using an httptest
	// LocalServer to mock requests from a real dynamic client
	localServer := createLocalTestServer(t)

	// parse the URL of the server to generate the KubeConfig file
	parsedURL, err := url.Parse(localServer.URL)
	if err != nil {
		t.Fatalf("failed to parse test server url %s", localServer.URL)
	}

	// ensure there is a valid kubeconfig in a tmp file for the dynamic dg
	kubeConfigPath, err := createKubeConfigWithServer(parsedURL.String())
	if err != nil {
		t.Fatalf("failed to create temp kubeconfig: %s", err)
	}
	defer os.Remove(kubeConfigPath)

	// ensure there is a valid host file, this would be loaded from a secret
	// mount in an agent pod
	hostConfigPath, err := createDgHostConfigWithServer("http://" + parsedURL.Host)
	if err != nil {
		t.Fatalf("failed to create temp kubeconfig: %s", err)
	}
	defer os.Remove(hostConfigPath)

	// create the config for the DataGatherer, wraps config for Dynamic client
	// and version checker
	textCfg := fmt.Sprintf(`
k8s:
  kubeconfig: %s
registries:
- kind: selfhosted
  params:
    host: %s
    bearer: fixtures/example_secret
`, kubeConfigPath, hostConfigPath)

	config := Config{}
	ctx := context.Background()
	err = yaml.Unmarshal([]byte(textCfg), &config)
	if err != nil {
		t.Fatalf("failed to load config: %+v", err)
	}

	dg, err := config.NewDataGatherer(ctx)
	if err != nil {
		t.Fatalf("failed create new dg %s", err)
	}

	stopChannelContext, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	vcDg := dg.(*DataGatherer)

	err = vcDg.Run(stopChannelContext.Done())
	if err != nil {
		t.Fatalf("unexpected dg Run error: %+v", err)
	}

	err = vcDg.WaitForCacheSync(stopChannelContext.Done())
	if err != nil {
		t.Fatalf("unexpected dg WaitForCacheSync error: %+v", err)
	}

	rawResults, err := vcDg.Fetch()
	if err != nil {
		t.Fatalf("failed fetch data: %s", err)
	}

	resultsJSON, err := json.MarshalIndent(rawResults, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal data: %s", err)
	}

	expectedResultsJSON := fmt.Sprintf(`[
  {
    "pod": {
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
        ],
        "nodeName": "control-plane"
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
    "results": [
      {
        "container_name": "example",
        "init_container": false,
        "result": {
          "CurrentVersion": "v1.0.0",
          "LatestVersion": "v1.0.1",
	  "IsLatest": false,
	  "ImageURL": "%s/jetstack/example",
	  "OS": "",
	  "Architecture": "amd64"
        }
      }
    ]
  }
]`, parsedURL.Host, parsedURL.Host)

	act := removeWhitespace(string(resultsJSON))
	expected := removeWhitespace(expectedResultsJSON)
	if act != expected {
		t.Fatalf("results json does not match: %s vs %s", act, expected)
	}
}

// config must be loaded from file paths, this creates a tmp file with the host
// to load in for the DataGatherer
func createDgHostConfigWithServer(server string) (string, error) {
	tmpfile, err := ioutil.TempFile("", tmpFilePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create a tmpfile for host")
	}

	if _, err := tmpfile.Write([]byte(server)); err != nil {
		return "", fmt.Errorf("failed to write to tmp host file")
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("failed to close tmp host file after writing")
	}

	return tmpfile.Name(), nil
}

// creates a kubeconfig file on disk with a reference to the local server
// allowing requests to be mocked
func createKubeConfigWithServer(server string) (string, error) {
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
    password: test`, server)
	tmpfile, err := ioutil.TempFile("", tmpFilePrefix)
	if err != nil {
		return "", fmt.Errorf("failed to create a tmpfile for kubeconfig")
	}

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return "", fmt.Errorf("failed to write to tmp kubeconfig file")
	}
	if err := tmpfile.Close(); err != nil {
		return "", fmt.Errorf("failed to close tmp kubeconfig file after writing")
	}

	return tmpfile.Name(), nil
}

// create a local test server to respond to k8s and registry api requests from
// the DataGatherer during operation. The dg is configured to use this local
// address to get data from k8s and registries.
func createLocalTestServer(t *testing.T) *httptest.Server {
	var localServer *httptest.Server
	localServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var responseContent []byte
		var err error

		switch r.URL.Path {
		case "/api/v1/nodes":
			responseContent, err = ioutil.ReadFile("fixtures/nodes.json")
			if err != nil {
				t.Fatalf("failed to read nodes fixture: %s", err)
			}
		case "/api/v1/pods":
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
		case "/v2/jetstack/example/tags/list":
			responseContent, err = ioutil.ReadFile("fixtures/tags.json")
			if err != nil {
				t.Fatalf("failed to read tags fixture: %s", err)
			}
		case "/v2/jetstack/example/manifests/v1.0.0":
			// this is a partial response, but it's all version checker needs
			// selfhosted registers currently don't expose OS information
			responseContent = []byte(`{
			  "schemaVersion": 1,
			  "name": "jetstack/example",
			  "tag": "v1.0.0",
			  "architecture":"amd64",
			  "os":""
			}`)
		case "/v2/jetstack/example/manifests/v1.0.1":
			// this is a partial response, but it's all version checker needs
			// selfhosted registers currently don't expose OS information
			responseContent = []byte(`{
			  "schemaVersion": 1,
			  "name": "jetstack/example",
			  "tag": "v1.0.1",
			  "architecture":"amd64",
			  "os":""
			}`)
		default:
			t.Fatalf("Unexpected URL was called: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseContent)
	}))

	return localServer
}

// Simple helper function to remove whitespaces from string
func removeWhitespace(input string) string {
	ws := regexp.MustCompile(`[\s]`)
	return ws.ReplaceAllString(input, "")
}
