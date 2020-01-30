package exporter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jetstack/preflight/api"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

func TestRawExportIndex(t *testing.T) {
	// TODO this is a copy of the json exporter function - consider making a
	// serialize method on the ClusterSummary struct
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

	exporter := NewRawExporter()
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
