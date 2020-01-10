package results

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"encoding/json"

	"github.com/open-policy-agent/opa/rego"
)

type outputResult map[string][]string

// Result holds the information about the result of a check
type Result struct {
	ID         string
	Violations []string
	Value      interface{} // deprecated
	Package    string
}

// IsSuccessState returns true if Value is boolean and it is true.
func (r *Result) IsSuccessState() bool {
	if r.Violations != nil {
		return len(r.Violations) == 0
	}

	success, ok := r.Value.(bool)
	if ok {
		log.Println("Using a boolean for `Value` is deprecated")
		return success
	} else {
		return false
	}
}

// IsFailureState returns true if Value is boolean and it is false.
func (r *Result) IsFailureState() bool {
	if r.Violations != nil {
		return len(r.Violations) != 0
	}

	success, ok := r.Value.(bool)
	if ok {
		log.Println("Using a boolean for `Value` is deprecated")
		return !success
	} else {
		return false
	}
}

// ResultCollection is a collection of Result
type ResultCollection []*Result

// NewResultCollection returns an empty ResultCollection.
func NewResultCollection() *ResultCollection {
	return &ResultCollection{}
}

// ListFailing returns a subset of the results that have failed.
func (r *ResultCollection) ListFailing() []*Result {
	results := []*Result{}
	for _, candidate := range *r {
		if candidate.IsFailureState() {
			results = append(results, candidate)
		}
	}
	return results
}

// ListPassing returns a subset of the results that have passed.
func (r *ResultCollection) ListPassing() []*Result {
	results := []*Result{}
	for _, candidate := range *r {
		if candidate.IsSuccessState() {
			results = append(results, candidate)
		}
	}
	return results
}

// Add adds a slice of results to the collection.
func (r *ResultCollection) Add(rr []*Result) {
	*r = append(*r, rr...)
}

// ByID returns a map of results by ID.
func (r *ResultCollection) ByID() map[string]*Result {
	resultMap := make(map[string]*Result)
	for _, result := range *r {
		resultMap[result.ID] = result
	}
	return resultMap
}

// NewResultCollectionFromRegoResultSet creates a new ResultCollection from a rego.ResultSet.
func NewResultCollectionFromRegoResultSet(rs *rego.ResultSet) (*ResultCollection, error) {
	if len(*rs) != 1 {
		return nil, errors.New("ResultSet does not contain 1 exact element")
	}
	if len((*rs)[0].Expressions) != 1 {
		return nil, errors.New("'expressions' does not contain exactly 1 element")
	}

	expression := (*rs)[0].Expressions[0]
	pkg := strings.TrimPrefix(expression.Text, "data.")
	values, ok := expression.Value.(map[string]interface{})
	if !ok {
		return nil, errors.New("format error, cannot unmarshall 'value'")
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rc := make(ResultCollection, 0, len(keys))
	for _, k := range keys {
		var violations []string
		boolValid, boolOk := values[k].(bool)
		strings, stringsOk := values[k].([]string)
		if boolOk {
			log.Printf("Using a boolean for `Value` is deprecated, found for key: (%s)", k)
			if boolValid {
				violations = []string{}
			} else {
				violations = []string{"missing violation context in rule definition"}
			}
		} else if stringsOk {
			violations = strings
		} else {
			return nil, fmt.Errorf("format error, cannot unmarshall value '%+v' to bool or []string", values[k])
		}
		rc = append(rc, &Result{
			ID:         k,
			Value:      violations,
			Violations: violations,
			Package:    pkg,
		})
	}

	return &rc, nil
}

// Parse takes the raw result of evaluating a set of rego rules in preflight and returns a ResultCollection collection.
func Parse(rawResult []byte) (*ResultCollection, error) {
	// parse raw data with opa.rego package
	data := outputResult{}
	err := json.Unmarshal(rawResult, &data)
	if err != nil {
		return nil, err
	}

	keys := make([]string, len(data))
	i := 0
	for k := range data {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	results := make([]*Result, len(keys))
	for idx, k := range keys {
		idChunks := strings.Split(k, "/")

		id := ""
		pkg := ""
		if n := len(idChunks); n == 1 {
			// We allow the use of no package
			id = idChunks[0]
		} else if n == 2 {
			pkg = idChunks[0]
			id = idChunks[1]
		} else {
			return nil, fmt.Errorf("cannot decode ID: %q", k)
		}
		results[idx] = &Result{
			ID:      id,
			Value:   data[k],
			Package: pkg,
		}
	}

	rc := ResultCollection(results)
	return &rc, nil
}

// Serialize serializes a ResultCollection into a JSON representation and writes it.
func (r *ResultCollection) Serialize(w io.Writer) error {
	output := make(outputResult, len(*r))

	for _, result := range *r {
		if len(result.Package) == 0 {
			return fmt.Errorf("missing Package in result with ID: %q", result.ID)
		}
		output[fmt.Sprintf("%s/%s", result.Package, result.ID)] = result.Violations
	}

	e := json.NewEncoder(w)
	err := e.Encode(output)
	if err != nil {
		return err
	}

	return nil
}
