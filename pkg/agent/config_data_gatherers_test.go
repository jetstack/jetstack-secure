package agent

import (
	"context"
	"reflect"
	"testing"
)

func TestUnknownDataGathererKind(t *testing.T) {
	ctx := context.Background()

	config := []byte{}

	_, err := LoadDataGatherer(ctx, "unknown", config)
	if err == nil {
		t.Fatalf("Expected an error when unknown data gatherer kind, no error returned")
	}

	if err.Error() != "cannot load data gatherer, kind 'unknown' is not supported" {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidLoadDataGatherer(t *testing.T) {
	ctx := context.Background()

	config := []byte(`param-1: "bar"`)

	dg, err := LoadDataGatherer(ctx, "dummy", config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if got, want := reflect.TypeOf(dg), reflect.TypeOf((*dummyDataGatherer)(nil)); got != want {
		t.Fatalf("DataGatherer type does not match: got=%v, want=%v", got, want)
	}

	dummyDG, ok := dg.(*dummyDataGatherer)
	if !ok {
		t.Fatalf("got a DataGatherer that is not a dummyDataGatherer")
	}

	if got, want := dummyDG.Param1, "bar"; got != want {
		t.Fatalf("DataGatherer does not contain the expected properties: got Param1=%v, want Param1=%v", got, want)
	}
}
