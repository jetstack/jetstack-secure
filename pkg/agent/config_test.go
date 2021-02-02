package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kylelemons/godebug/diff"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestValidConfigLoad(t *testing.T) {
	configFileContents := `
      server: "http://localhost:8080"
      period: 1h
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
      - name: d1
        kind: dummy
        config:
          always-fail: false
      input-path: "/home"
      output-path: "/nothome"
`

	loadedConfig, err := ParseConfig([]byte(configFileContents))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := Config{
		Server:         "http://localhost:8080",
		Period:         time.Hour,
		OrganizationID: "example",
		ClusterID:      "example-cluster",
		DataGatherers: []dataGatherer{
			{
				Name: "d1",
				Kind: "dummy",
				Config: &dummyConfig{
					AlwaysFail: false,
				},
			},
		},
		InputPath:  "/home",
		OutputPath: "/nothome",
	}

	if diff, equal := messagediff.PrettyDiff(expected, loadedConfig); !equal {
		t.Errorf("Diff %s", diff)
	}
}

func TestValidConfigWithEndpointLoad(t *testing.T) {
	configFileContents := `
      endpoint:
        host: example.com
        path: api/v1/data
      schedule: "* * * * *"
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
      - name: d1
        kind: dummy
        config:
          always-fail: false
`

	loadedConfig, err := ParseConfig([]byte(configFileContents))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := Config{
		Endpoint: Endpoint{
			Protocol: "http",
			Host:     "example.com",
			Path:     "api/v1/data",
		},
		Schedule:       "* * * * *",
		OrganizationID: "example",
		ClusterID:      "example-cluster",
		DataGatherers: []dataGatherer{
			dataGatherer{
				Name: "d1",
				Kind: "dummy",
				Config: &dummyConfig{
					AlwaysFail: false,
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
		"2 errors occurred:",
		"\t* organization_id is required",
		"\t* cluster_id is required",
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
      organization_id: "example"
      cluster_id: "example-cluster"
      data-gatherers:
        - kind: dummy`))

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"1 error occurred:",
		"\t* datagatherer 1/1 is missing a name",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestInvalidServerError(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      server: "something not a URL"
      organization_id: "my_org"
      cluster_id: "my_cluster"
      data-gatherers:
        - kind: dummy
          name: dummy`))

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	expectedErrorLines := []string{
		"1 error occurred:",
		"\t* server is not a valid URL",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := parseError.Error()

	if gotError != expectedError {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestInvalidDataGathered(t *testing.T) {
	_, parseError := ParseConfig([]byte(`
      endpoint:
        host: example.com
        path: /api/v1/data
      schedule: "* * * * *"
      data-gatherers:
        - kind: "foo"`))

	if parseError == nil {
		t.Fatalf("expected error, got nil")
	}

	if got, want := parseError.Error(), `cannot parse data-gatherer configuration, kind "foo" is not supported`; got != want {
		t.Errorf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", got, want, diff.Diff(got, want))
	}
}
