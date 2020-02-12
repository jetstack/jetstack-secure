package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kylelemons/godebug/diff"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestValidConfigLoad(t *testing.T) {
	configFileContents := `
      endpoint:
        host: example.com
        path: api/v1/data
      schedule: "* * * * *"
      token: "12345"
      data-gatherers:
      - name: my-gke-cluster
        kind: gke
        data:
          project: my-gcp-project
          location: us-central1-a
          cluster: my-gke-cluster
      - name: my-pods
        kind: k8s/pods
        data:
          kubeconfig: "~/.kube/config"`

	loadedConfig, err := ParseConfig([]byte(configFileContents))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := Config{
		Endpoint: Endpoint{
			Protocol: "http",
			Host:     "example.com",
			Path:     "/api/v1/data",
		},
		Schedule: "* * * * *",
		Token:    "12345",
		DataGatherers: []dataGatherer{
			dataGatherer{
				Name: "my-gke-cluster",
				Kind: "gke",
				Data: map[string]string{
					"project":  "my-gcp-project",
					"location": "us-central1-a",
					"cluster":  "my-gke-cluster",
				},
			},
			dataGatherer{
				Name: "my-pods",
				Kind: "k8s/pods",
				Data: map[string]string{
					"kubeconfig": "~/.kube/config",
				},
			},
		},
	}

	if diff, equal := messagediff.PrettyDiff(expected, loadedConfig); !equal {
		t.Errorf("Diff %s", diff)
	}
}

func TestInvalidConfigError(t *testing.T) {
	configFileContents := `data-gatherers: "things"`

	_, parseError := ParseConfig([]byte(configFileContents))

	expectedError := fmt.Errorf("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `things` into []agent.dataGatherer")

	if parseError.Error() != expectedError.Error() {
		t.Fatalf("got != want;\ngot=%s,\nwant=%s", parseError, expectedError)
	}
}

func TestMissingConfigError(t *testing.T) {
	_, parseError := ParseConfig([]byte(""))

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"4 errors occurred:",
		"\t* token is required",
		"\t* schedule is required",
		"\t* endpoint host is required",
		"\t* endpoint path is required",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestPartialMissingConfigError(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      endpoint:
        host: example.com
        path: /api/v1/data
      schedule: "* * * * *"
      token: "12345"
      data-gatherers:
        - foo: bar`))

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"2 errors occurred:",
		"\t* datagatherer 1/1 is missing a kind",
		"\t* datagatherer 1/1 is missing a name",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}
