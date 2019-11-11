package results

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

func TestIsSuccessState(t *testing.T) {
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
					"_1_4_3": true,
					"_1_4_4": false,
					"_1_4_5": true,
					"_1_4_6": true,
					"node_pools_with_legacy_endpoints_enabled": []interface{}{},
					"node_pools_without_cloud_platform_scope":  []string{"default-pool"},
				},
				Text: "data.package.name",
			}}},
		}

		expectedResults := []*Result{
			&Result{ID: "_1_4_3", Value: true, Package: "package.name"},
			&Result{ID: "_1_4_4", Value: false, Package: "package.name"},
			&Result{ID: "_1_4_5", Value: true, Package: "package.name"},
			&Result{ID: "_1_4_6", Value: true, Package: "package.name"},
			&Result{ID: "node_pools_with_legacy_endpoints_enabled", Value: []interface{}{}, Package: "package.name"},
			&Result{ID: "node_pools_without_cloud_platform_scope", Value: []string{"default-pool"}, Package: "package.name"},
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
  %q: false,
  "my.package/_1_4_3": true
}
		`, id)))

			if got, want := err, fmt.Errorf("cannot decode ID: %q", id); !errorsEqual(got, want) {
				t.Fatalf("got != want: got=%+v, want=%+v", got, want)
			}
		}
	})

	t.Run("parses a valid input", func(t *testing.T) {
		results := []*Result{
			&Result{ID: "_1_4_3", Value: true},
			&Result{ID: "_1_4_4", Value: false, Package: "my.package"},
			&Result{ID: "node_pools_without_cloud_platform_scope", Value: []interface{}{"default-pool"}},
		}

		rc, err := Parse([]byte(`
{
  "node_pools_without_cloud_platform_scope": [
	"default-pool"
  ],
  "_1_4_3": true,
  "my.package/_1_4_4": false
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

func TestListFailing(t *testing.T) {
	a := &Result{ID: "a", Value: true}
	b := &Result{ID: "b", Value: false}
	c := &Result{ID: "c", Value: true}
	d := &Result{ID: "d", Value: false}
	e := &Result{ID: "e", Value: []string{}}
	f := &Result{ID: "f", Value: []string{"something"}}

	rc := &ResultCollection{a, b, c, d, e, f}

	expected := []*Result{b, d}

	if got, want := rc.ListFailing(), expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestListPassing(t *testing.T) {
	a := &Result{ID: "a", Value: true}
	b := &Result{ID: "b", Value: false}
	c := &Result{ID: "c", Value: true}
	d := &Result{ID: "d", Value: false}
	e := &Result{ID: "e", Value: []string{}}
	f := &Result{ID: "f", Value: []string{"something"}}

	rc := &ResultCollection{a, b, c, d, e, f}

	expected := []*Result{a, c}

	if got, want := rc.ListPassing(), expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestAdd(t *testing.T) {
	a := &Result{ID: "a", Value: false}
	b := &Result{ID: "b", Value: true}
	c := &Result{ID: "c", Value: false}
	d := &Result{ID: "d", Value: true}

	rc := NewResultCollection()
	rc.Add([]*Result{a, b})
	rc.Add([]*Result{c, d})

	expected := []*Result{a, b, c, d}

	if got, want := []*Result(*rc), expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestByID(t *testing.T) {
	a := &Result{ID: "a", Value: false}
	b := &Result{ID: "b", Value: true}
	c := &Result{ID: "c", Value: false}
	d := &Result{ID: "d", Value: true}

	rc := &ResultCollection{a, b, c, d}

	rByID := rc.ByID()

	expected := map[string]*Result{"a": a, "b": b, "c": c, "d": d}

	if got, want := rByID, expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}

func TestSerialize(t *testing.T) {
	t.Run("returns error if Package if empty", func(t *testing.T) {
		rc := &ResultCollection{
			&Result{ID: "a", Value: true, Package: ""},
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
			&Result{ID: "a", Value: true, Package: "p1"},
			&Result{ID: "b", Value: false, Package: "p1"},
			&Result{ID: "c", Value: []string{}, Package: "p1"},
			&Result{ID: "d", Value: []string{"something"}, Package: "p2"},
		}

		expectedSerialization := `{"p1/a":true,"p1/b":false,"p1/c":[],"p2/d":["something"]}
`

		var buf bytes.Buffer
		err := rc.Serialize(&buf)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if got, want := buf.String(), expectedSerialization; got != want {
			t.Fatalf("got!=want: got=%s, want=%s", got, want)
		}
	})
}
