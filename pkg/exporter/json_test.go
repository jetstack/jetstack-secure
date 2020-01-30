package exporter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/reports"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
)

func TestJSONExport(t *testing.T) {
	pm := &packaging.PolicyManifest{
		SchemaVersion:  "0.1.1",
		Namespace:      "test.org",
		ID:             "test-pkg",
		PackageVersion: "1.2.3",
		Name:           "Test Package",
		Description:    "This is a test package.",
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
				},
			},
			packaging.Section{
				ID:          "section-2",
				Name:        "Sample section 2",
				Description: "This is another section.",
				Rules: []packaging.Rule{
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
		&results.Result{ID: rules.RuleToResult("r1"), Violations: []string{}},
		&results.Result{ID: rules.RuleToResult("r2"), Violations: []string{"violation"}},
		&results.Result{ID: "preflight_r3", Violations: []string{"another violation"}},
	}

	expectedJSON := `{
  "sections": [
    {
      "rules": [
        {
          "missing": false,
          "success": true,
          "violations": [],
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
          "violations": [
            "violation"
          ],
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is another rule.",
          "name": "Another rule",
          "id": "r2"
        },
        {
          "missing": true,
          "success": false,
          "violations": [],
          "links": [
            "http://jetstack.io/docs",
            "http://jetstack.io/docs2"
          ],
          "remediation": "No remedy.",
          "description": "This is another rule.",
          "name": "Another rule",
          "id": "r3"
        }
      ],
      "description": "This is a section.",
      "name": "Sample section",
      "id": "section-1"
    },
    {
      "rules": [
        {
          "missing": true,
          "success": false,
          "violations": [],
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
  "package-information": {
    "id": "test-pkg",
    "namespace": "test.org",
    "version": "1.2.3",
    "schema-version": "0.1.1"
  },
  "preflight-version": "development",
  "cluster": "",
  "timestamp": "0001-01-01T00:00:00Z",
  "id": ""
}`

	buf, err := jsonExporter.Export(context.Background(), pm, nil, rc)
	if _, ok := err.(*reports.MissingRegoDefinitionError); !ok {
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

func TestJSONExportBackwardsCompatibility(t *testing.T) {
	pm := &packaging.PolicyManifest{
		SchemaVersion:  "0.1.0",
		Namespace:      "test.org",
		ID:             "test-pkg",
		PackageVersion: "1.2.3",
		Name:           "Test Package",
		Description:    "This is a test package.",
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
		},
	}

	jsonExporter := JSONExporter{}

	rc := &results.ResultCollection{
		&results.Result{ID: rules.RuleToResult("r1"), Violations: []string{}},
		&results.Result{ID: rules.LegacyRuleToResult("r2"), Violations: []string{"violation"}},
	}

	expectedJSON := `{
  "sections": [
    {
      "rules": [
        {
          "missing": false,
          "success": true,
          "violations": [],
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
          "violations": [
            "violation"
          ],
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
    }
  ],
  "description": "This is a test package.",
  "name": "Test Package",
  "package": "test-pkg",
  "package-information": {
    "id": "test-pkg",
    "namespace": "test.org",
    "version": "1.2.3",
    "schema-version": "0.1.0"
  },
  "preflight-version": "development",
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

func TestJSONExportIndex(t *testing.T) {
	clusterSummary := api.ClusterSummary{
		Cluster: "exampleCluster",
		LatestReportSet: &api.ReportSet{
			Cluster:      "exampleCluster",
			Timestamp:    api.Time{},
			FailureCount: 4,
			SuccessCount: 1,
			Reports: []*api.ReportSummary{
				&api.ReportSummary{
					ID:           "exampleReport1",
					Package:      "examplePackage.ID.1",
					Cluster:      "exampleCluster",
					Timestamp:    api.Time{},
					FailureCount: 2,
					SuccessCount: 1,
				},
				&api.ReportSummary{
					ID:           "exampleReport2",
					Package:      "examplePackage.ID.2",
					Cluster:      "exampleCluster",
					Timestamp:    api.Time{},
					FailureCount: 2,
					SuccessCount: 0,
				},
			},
		},
	}

	exporter := NewJSONExporter()
	var ctx context.Context

	actualJSON, err := exporter.ExportIndex(ctx, &clusterSummary)
	if err != nil {
		t.Errorf("error exporting: %v", err)
	}

	expectedJSON := `{
  "cluster": "exampleCluster",
  "latestReportSet": {
    "timestamp": "0001-01-01T00:00:00Z",
    "failureCount": 4,
    "successCount": 1,
    "reports": [
      {
        "id": "exampleReport1",
        "package": "examplePackage.ID.1",
        "failureCount": 2,
        "successCount": 1
      },
      {
        "id": "exampleReport2",
        "package": "examplePackage.ID.2",
        "failureCount": 2,
        "successCount": 0
      }
    ]
  }
}`

	var got, want map[string]interface{}

	if err = json.Unmarshal([]byte(expectedJSON), &want); err != nil {
		t.Fatalf("%+v", err)
	}

	if err = json.Unmarshal(actualJSON.Bytes(), &got); err != nil {
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
