package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jetstack/preflight/pkg/datagatherer/gke"
	"github.com/jetstack/preflight/pkg/datagatherer/local"
	"github.com/kylelemons/godebug/diff"
	"gopkg.in/d4l3k/messagediff.v1"
)

func TestInvalidDatagathererMissingKind(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		// e.g. "kind": "gke" is missing
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected an error when missing kind, no error returned")
	}

	if err.Error() != "cannot load data gatherer, config is missing 'kind' attribute and so data gatherer kind cannot be determined" {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestUnknownDataGathererKind(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "unknown",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected an error when unknown data gatherer kind, no error returned")
	}

	if err.Error() != "cannot load data gatherer, kind 'unknown' is not supported" {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidGKEConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind":        "gke",
		"project":     "example-project",
		"location":    "us-east1-b",
		"cluster":     "main",
		"credentials": "path_to_creds",
	}

	cluster := gke.Cluster{
		Project:  "example-project",
		Name:     "main",
		Location: "us-east1-b",
	}

	expected := gke.NewGKEDataGatherer(ctx, &cluster, "path_to_creds")

	dg, err := LoadDataGatherer(ctx, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if diff, equal := messagediff.PrettyDiff(dg, expected); !equal {
		fmt.Printf("got %v\n", dg)
		t.Fatalf("Diff %s", diff)
	}
}

func TestInValidGKEConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "gke",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected an error when given invalid config, no error returned")
	}

	expectedErrorLines := []string{
		"failed to load GKE DataGatherer: 4 errors occurred:",
		"\t* project is required",
		"\t* cluster is required",
		"\t* location is required",
		"\t* credentials is required",
		"\n",
	}

	expectedError := strings.Join(expectedErrorLines, "\n")

	gotError := err.Error()

	if gotError != expectedError {
		t.Fatalf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", gotError, expectedError, diff.Diff(gotError, expectedError))
	}
}

func TestInValidK8sConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "k8s/pod",
		// "kubeconfig": "missing",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if err.Error() != "failed to load K8s DataGatherer: kubeconfig path is required" {
		t.Fatalf("Unxpected error, %s", err.Error())
	}
}

func TestInValidEKSConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "eks",
		// "cluster": "missing",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if err.Error() != "failed to load EKS DataGatherer: cluster is required" {
		t.Fatalf("Unxpected error, %s", err.Error())
	}
}

func TestInValidAKSConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "aks",
		// "cluster": "missing",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	expectedError := "failed to load AKS DataGatherer: 3 errors occurred:\n" +
		"\t* project is required\n" +
		"\t* cluster is required\n" +
		"\t* location is required\n\n"

	if err.Error() != expectedError {
		t.Fatalf("\ngot=\n%v\nwant=\n%s\ndiff=\n%s", err.Error(), expectedError, diff.Diff(err.Error(), expectedError))
	}
}

func TestValidLocalConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind":      "local",
		"data-path": "dump.json",
	}

	expected := local.NewLocalDataGatherer("dump.json")

	dg, err := LoadDataGatherer(ctx, config)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if diff, equal := messagediff.PrettyDiff(dg, expected); !equal {
		fmt.Printf("got %v\n", dg)
		t.Fatalf("Diff %s", diff)
	}
}

func TestInValidLocalConfig(t *testing.T) {
	ctx := context.Background()

	config := map[string]string{
		"kind": "local",
		// "data-path": "missing",
	}

	_, err := LoadDataGatherer(ctx, config)
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if err.Error() != "failed to load Local DataGatherer: data-path is required" {
		t.Fatalf("Unxpected error, %s", err.Error())
	}
}
