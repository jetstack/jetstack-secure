package results

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

func TestIsSuccessState(t *testing.T) {
	testCases := []struct {
		result *Result
		want   bool
	}{
		{&Result{ID: "1", Violations: []string{}}, true},
		{&Result{ID: "2", Violations: []string{"violation"}}, false},
	}

	for idx, tc := range testCases {
		t.Run(string(idx), func(t *testing.T) {
			if got, want := tc.result.IsSuccessState(), tc.want; got != want {
				t.Fatalf("got!=want: got=%v, want=%v", got, want)
			}
		})
	}
}

func TestIsSuccessStateWithLegacyValueResult(t *testing.T) {
	testCases := []struct {
		result *Result
		want   bool
	}{
		{&Result{ID: "1", Value: ""}, false},
		{&Result{ID: "2", Value: "aaa"}, false},
		{&Result{ID: "3", Value: []interface{}{}}, false},
		{&Result{ID: "4", Value: []string{"aaa"}}, false},
		{&Result{ID: "5", Value: true}, true},
		{&Result{ID: "6", Value: false}, false},
	}

	for idx, tc := range testCases {
		t.Run(string(idx), func(t *testing.T) {
			if got, want := tc.result.IsSuccessState(), tc.want; got != want {
				t.Fatalf("got!=want: got=%v, want=%v", got, want)
			}
		})
	}
}

func TestIsFailureState(t *testing.T) {
	testCases := []struct {
		result *Result
		want   bool
	}{
		{&Result{ID: "1", Violations: []string{}}, false},
		{&Result{ID: "2", Violations: []string{"violation"}}, true},
		{&Result{ID: "3", Violations: []string{"violation", "more violation"}}, true},
	}

	for idx, tc := range testCases {
		t.Run(string(idx), func(t *testing.T) {
			if got, want := tc.result.IsFailureState(), tc.want; got != want {
				t.Fatalf("got!=want: got=%v, want=%v", got, want)
			}
		})
	}
}

func TestIsFailureStateWithLegacyValueResult(t *testing.T) {
	testCases := []struct {
		result *Result
		want   bool
	}{
		{&Result{ID: "1", Value: ""}, false},
		{&Result{ID: "2", Value: "aaa"}, false},
		{&Result{ID: "3", Value: []interface{}{}}, false},
		{&Result{ID: "4", Value: []string{"aaa"}}, false},
		{&Result{ID: "5", Value: true}, false},
		{&Result{ID: "6", Value: false}, true},
	}

	for idx, tc := range testCases {
		t.Run(string(idx), func(t *testing.T) {
			if got, want := tc.result.IsFailureState(), tc.want; got != want {
				t.Fatalf("got!=want: got=%v, want=%v", got, want)
			}
		})
	}
}

func TestNewResultCollectionFromRegoResultSet(t *testing.T) {
	err1 := "ResultSet does not contain 1 exact element"
	err2 := "'expressions' does not contain exactly 1 element"
	err3 := "format error, cannot unmarshall 'value'"

	testCases := []struct {
		input   *rego.ResultSet
		wantErr error
	}{
		{
			&rego.ResultSet{rego.Result{}, rego.Result{}},
			errors.New(err1),
		},
		{
			&rego.ResultSet{rego.Result{}},
			errors.New(err2),
		},
		{
			&rego.ResultSet{rego.Result{
				Expressions: []*rego.ExpressionValue{},
			}},
			errors.New(err2),
		},
		{
			&rego.ResultSet{rego.Result{
				Expressions: []*rego.ExpressionValue{&rego.ExpressionValue{Value: ""}},
			}},
			errors.New(err3),
		},
	}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("returns error on wrong format %d", idx), func(t *testing.T) {
			_, err := NewResultCollectionFromRegoResultSet(tc.input)
			if got, want := err, tc.wantErr; !errorsEqual(got, want) {
				t.Fatalf("got != want: got=%+v, want=%+v", got, want)
			}
		})
	}

	t.Run("parses a valid input", func(t *testing.T) {
		regoResultSet := &rego.ResultSet{
			rego.Result{Expressions: []*rego.ExpressionValue{&rego.ExpressionValue{
				Value: map[string]interface{}{
					"_1_4_3": []interface{}{},
					"_1_4_4": []interface{}{"violation"},
					"_1_4_5": []interface{}{},
					"_1_4_6": []interface{}{},
					"node_pools_with_legacy_endpoints_enabled": []interface{}{},
					"node_pools_without_cloud_platform_scope":  []interface{}{"violation"},
					"something_returning_a_map":                []interface{}{map[string]string{"bar": "foo"}},
				},
				Text: "data.package.name",
			}}},
		}

		expectedResults := []*Result{
			&Result{ID: "_1_4_3", Value: []string{}, Violations: []string{}, Package: "package.name"},
			&Result{ID: "_1_4_4", Value: []string{"violation"}, Violations: []string{"violation"}, Package: "package.name"},
			&Result{ID: "_1_4_5", Value: []string{}, Violations: []string{}, Package: "package.name"},
			&Result{ID: "_1_4_6", Value: []string{}, Violations: []string{}, Package: "package.name"},
			&Result{ID: "node_pools_with_legacy_endpoints_enabled", Value: []string{}, Violations: []string{}, Package: "package.name"},
			&Result{ID: "node_pools_without_cloud_platform_scope", Value: []string{"violation"}, Violations: []string{"violation"}, Package: "package.name"},
			&Result{ID: "something_returning_a_map", Value: []string{fmt.Sprintf("%+v", map[string]string{"bar": "foo"})}, Violations: []string{fmt.Sprintf("%+v", map[string]string{"bar": "foo"})}, Package: "package.name"},
		}

		rc, err := NewResultCollectionFromRegoResultSet(regoResultSet)
		if err != nil {
			t.Fatalf("Unexpected error: %+v", err)
		}

		if got, want := len(*rc), len(expectedResults); got != want {
			t.Fatalf("Wrong length of result: got=%+v, want=%+v", got, want)
		}

		for idx, r := range *rc {
			if got, want := r, expectedResults[idx]; !reflect.DeepEqual(got, want) {
				t.Fatalf("got != want: got=%+v, want=%+v", got, want)
			}
		}
	})
}

func TestParse(t *testing.T) {
	t.Run("returns error on wrong JSON", func(t *testing.T) {
		_, err := Parse([]byte{})
		if got, want := err, errors.New("unexpected end of JSON input"); !errorsEqual(got, want) {
			t.Fatalf("got != want: got=%+v, want=%+v", got, want)
		}
	})

	t.Run("returns error if badformat ID", func(t *testing.T) {
		badIDs := []string{
			"a/a/_1_4_4",
			"/a/_1_4_4",
			"//_1_4_4",
		}

		for _, id := range badIDs {
			_, err := Parse([]byte(fmt.Sprintf(`
{
  "node_pools_without_cloud_platform_scope": [
	"default-pool"
  ],
  %q: [],
  "my.package/_1_4_3": []
}
		`, id)))

			if got, want := err, fmt.Errorf("cannot decode ID: %q", id); !errorsEqual(got, want) {
				t.Fatalf("got != want: got=%+v, want=%+v", got, want)
			}
		}
	})

	t.Run("parses a valid input", func(t *testing.T) {
		results := []*Result{
			&Result{ID: "_1_4_3", Value: []string{}},
			&Result{ID: "_1_4_4", Value: []string{"violation"}, Package: "my.package"},
			&Result{ID: "node_pools_without_cloud_platform_scope", Value: []string{"default-pool"}},
		}

		rc, err := Parse([]byte(`
{
  "node_pools_without_cloud_platform_scope": [
	"default-pool"
  ],
  "_1_4_3": [],
  "my.package/_1_4_4": [
	"violation"
  ]
}
		`))
		if err != nil {
			t.Fatalf("Unexpected error: %+v", err)
		}

		if got, want := len(*rc), len(results); got != want {
			t.Fatalf("Wrong length of result: got=%+v, want=%+v", got, want)
		}

		for idx, r := range *rc {
			if got, want := r, results[idx]; !reflect.DeepEqual(got, want) {
				t.Fatalf("got != want: got=%+v, want=%+v", got, want)
			}
		}
	})
}

func errorsEqual(err1 error, err2 error) bool {
	if err1 == nil || err2 == nil {
		return err1 == err2
	}
	return err1.Error() == err2.Error()
}

func TestListPassing(t *testing.T) {
	a := &Result{ID: "a", Violations: []string{}}
	b := &Result{ID: "b", Violations: []string{"violation"}}

	rc := &ResultCollection{a, b}

	want := []*Result{a}

	got := rc.ListPassing()

	var gotResultIDs []string
	for _, v := range got {
		gotResultIDs = append(gotResultIDs, v.ID)
	}

	var wantedResultIDs []string
	for _, v := range want {
		wantedResultIDs = append(wantedResultIDs, v.ID)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", gotResultIDs, wantedResultIDs)
	}
}

func TestListFailing(t *testing.T) {
	a := &Result{ID: "a", Violations: []string{}}
	b := &Result{ID: "b", Violations: []string{"violation"}}

	rc := &ResultCollection{a, b}

	want := []*Result{b}

	got := rc.ListFailing()

	var gotResultIDs []string
	for _, v := range got {
		gotResultIDs = append(gotResultIDs, v.ID)
	}

	var wantedResultIDs []string
	for _, v := range want {
		wantedResultIDs = append(wantedResultIDs, v.ID)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", gotResultIDs, wantedResultIDs)
	}
}

func TestAdd(t *testing.T) {
	a := &Result{ID: "a", Violations: []string{}}
	b := &Result{ID: "b", Violations: []string{"violation"}}
	c := &Result{ID: "c", Violations: []string{"violation"}}

	rc := NewResultCollection()
	rc.Add([]*Result{a})
	rc.Add([]*Result{b, c})

	got := rc.ByID()
	want := map[string]*Result{"a": a, "b": b, "c": c}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestByID(t *testing.T) {
	a := &Result{ID: "a"}
	b := &Result{ID: "b"}

	rc := &ResultCollection{a, b}

	rByID := rc.ByID()

	expected := map[string]*Result{"a": a, "b": b}

	if got, want := rByID, expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestSerialize(t *testing.T) {
	t.Run("returns error if Package if empty", func(t *testing.T) {
		rc := &ResultCollection{
			&Result{ID: "a", Violations: []string{}, Package: ""},
		}

		var buf bytes.Buffer
		err := rc.Serialize(&buf)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got, want := err.Error(), `missing Package in result with ID: "a"`; got != want {
			t.Fatalf("got != want: got=%s, want=%s", got, want)
		}
	})

	t.Run("writes desired JSON serialization of ResultCollection", func(t *testing.T) {
		rc := &ResultCollection{
			&Result{ID: "a", Package: "p1", Violations: []string{}},
			&Result{ID: "b", Package: "p1", Violations: []string{"violation"}},
			&Result{ID: "c", Package: "p1", Violations: []string{}},
			&Result{ID: "d", Package: "p2", Violations: []string{"violation"}},
		}

		expectedSerialization := `{"p1/a":[],"p1/b":["violation"],"p1/c":[],"p2/d":["violation"]}`

		var buf bytes.Buffer
		err := rc.Serialize(&buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if got, want := strings.TrimSpace(buf.String()), expectedSerialization; got != want {
			t.Fatalf("got!=want: got=%s, want=%s", got, want)
		}
	})
}
