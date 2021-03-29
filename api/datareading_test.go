package api

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJSONGatheredResourceDropsEmptyTime(t *testing.T) {
	var resource GatheredResource
	bytes, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal %s", err)
	}

	expected := `{"resource":null}`

	if string(bytes) != expected {
		t.Fatalf("unexpected json \ngot  %s\nwant %s", string(bytes), expected)
	}
}

func TestJSONGatheredResourceSetsTimeWhenPresent(t *testing.T) {
	var resource GatheredResource
	resource.DeletedAt = Time{time.Date(2021, 3, 29, 0, 0, 0, 0, time.UTC)}
	bytes, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal %s", err)
	}

	expected := `{"resource":null,"deleted_at":"2021-03-29T00:00:00Z"}`

	if string(bytes) != expected {
		t.Fatalf("unexpected json \ngot  %s\nwant %s", string(bytes), expected)
	}
}
