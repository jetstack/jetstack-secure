package exporter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"

	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
)

func TestJSONExport(t *testing.T) {
	pm := &packaging.PolicyManifest{
		ID:          "test-pkg",
		Name:        "Test Package",
		Description: "This is a test package.",
		Sections: []packaging.Section{
			packaging.Section{
				ID:          "section-1",
				Name:        "Sample section",
				Description: "This is a section.",
				Rules: []packaging.Rule{
					packaging.Rule{
						ID:          "r1",
						Name:        "A rule",
						Description: "This is a rule.",
						Manual:      false,
						Remediation: "No remedy.",
						Links: []string{
							"http://jetstack.io/docs",
							"http://jetstack.io/docs2",
						},
					},
					packaging.Rule{
						ID:          "r2",
						Name:        "Another rule",
						Description: "This is another rule.",
						Manual:      false,
						Remediation: "No remedy.",
						Links: []string{
							"http://jetstack.io/docs",
							"http://jetstack.io/docs2",
						},
					},
				},
			},
			packaging.Section{
				ID:          "section-2",
				Name:        "Sample section 2",
				Description: "This is another section.",
				Rules: []packaging.Rule{
					packaging.Rule{
						ID:          "r3",
						Name:        "Another rule",
						Description: "This is another rule.",
						Manual:      false,
						Remediation: "No remedy.",
						Links: []string{
							"http://jetstack.io/docs",
							"http://jetstack.io/docs2",
						},
					},
					packaging.Rule{
						ID:          "r4",
						Name:        "Another rule",
						Description: "This is another rule.",
						Manual:      false,
						Remediation: "No remedy.",
						Links: []string{
							"http://jetstack.io/docs",
							"http://jetstack.io/docs2",
						},
					},
				},
			},
		},
	}

	jsonExporter := JSONExporter{}

	rc := &results.ResultCollection{
		&results.Result{ID: ruleToResult("r1"), Value: true},
		&results.Result{ID: ruleToResult("r2"), Value: false},
		&results.Result{ID: ruleToResult("r3"), Value: "not a bool"},
	}

	expectedJSON := `{
  "sections": [
    {
      "rules": [
        {
          "missing": false,
          "success": true,
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is a rule.",
          "name": "A rule",
          "id": "r1"
        },
        {
          "missing": false,
          "success": false,
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is another rule.",
          "name": "Another rule",
          "id": "r2"
        }
      ],
      "description": "This is a section.",
      "name": "Sample section",
      "id": "section-1"
    },
    {
      "rules": [
        {
          "missing": false,
		  "value": "not a bool",
          "success": false,
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is another rule.",
          "name": "Another rule",
          "id": "r3"
        },
        {
          "missing": true,
          "success": false,
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is another rule.",
          "name": "Another rule",
          "id": "r4"
        }
      ],
      "description": "This is another section.",
      "name": "Sample section 2",
      "id": "section-2"
    }
  ],
  "description": "This is a test package.",
  "name": "Test Package",
  "package": "test-pkg",
  "cluster": "",
  "timestamp": "0001-01-01T00:00:00Z",
  "id": ""
}`

	buf, err := jsonExporter.Export(context.Background(), pm, nil, rc)
	if err != nil {
		t.Fatalf("unexpected err: %+v", err)
	}

	var got, want map[string]interface{}

	if err = json.Unmarshal([]byte(expectedJSON), &want); err != nil {
		t.Fatalf("%+v", err)
	}

	if err = json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("%+v", err)
	}

	diff := gojsondiff.New().CompareObjects(want, got)

	if diff.Modified() {
		f := formatter.NewAsciiFormatter(want, formatter.AsciiFormatterConfig{ShowArrayIndex: true, Coloring: true})
		differences, err := f.Format(diff)
		if err != nil {
			t.Errorf("could not format diff: %+v", err)
		}

		t.Fatalf("got != want: %v", differences)
	}
}
