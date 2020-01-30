package exporter

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jetstack/preflight/api"
)

func TestCLIExportIndex(t *testing.T) {
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

	exporter := NewCLIExporter()
	var ctx context.Context

	got, err := exporter.ExportIndex(ctx, &clusterSummary)
	if err != nil {
		t.Errorf("error exporting: %v", err)
	}

	want := `Summary
-------
cluster: exampleCluster
failures: 4
successes: 1
reports:
  package: examplePackage.ID.1
    failures: 2
    successes: 1
  package: examplePackage.ID.2
    failures: 2
    successes: 0
`

	if diff := cmp.Diff(want, got.String()); diff != "" {
		t.Errorf("ExportIndex diff (-want +got):\n%s", diff)
	}
}
