package reports

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
	"github.com/jetstack/preflight/pkg/version"
)

var fixtureReport1 = api.Report{
	ID:               "fixtureReport1",
	PreflightVersion: "version.PreflightVersion",
	Package:          "examplePackage.ID",
	PackageInformation: api.PackageInformation{
		Namespace: "examplePackage.Namespace",
		ID:        "examplePackage.ID",
		Version:   "examplePackage.PackageVersion",
	},
	Name:        "examplePackage.Name",
	Description: "examplePackage.Description",
	Cluster:     "exampleCluster",
	Sections: []api.ReportSection{
		api.ReportSection{
			ID:   "a_section",
			Name: "My section",
			Rules: []api.ReportRule{
				api.ReportRule{
					ID:         "a_rule",
					Name:       "My Rule A",
					Manual:     false,
					Success:    true,
					Missing:    false,
					Links:      []string{},
					Violations: []string{},
				},
				api.ReportRule{
					ID:         "b_rule",
					Name:       "My Rule B",
					Manual:     false,
					Success:    false,
					Missing:    false,
					Links:      []string{},
					Violations: []string{"violation"},
				},
				api.ReportRule{
					ID:         "c_rule",
					Name:       "My Rule C (missing)",
					Manual:     false,
					Success:    false,
					Missing:    true,
					Links:      []string{},
					Violations: []string{},
				},
			},
		},
	},
}

var fixtureReport2 = api.Report{
	ID:               "fixtureReport2",
	PreflightVersion: "version.PreflightVersion",
	Package:          "examplePackage.ID",
	PackageInformation: api.PackageInformation{
		Namespace: "examplePackage.Namespace",
		ID:        "examplePackage.ID",
		Version:   "examplePackage.PackageVersion",
	},
	Name:        "examplePackage.Name",
	Description: "examplePackage.Description",
	Cluster:     "exampleCluster",
	Sections: []api.ReportSection{
		api.ReportSection{
			ID:   "a_section",
			Name: "My section",
			Rules: []api.ReportRule{
				api.ReportRule{
					ID:         "a_rule",
					Name:       "My Rule A",
					Manual:     false,
					Success:    false,
					Missing:    false,
					Links:      []string{},
					Violations: []string{"violation"},
				},
				api.ReportRule{
					ID:         "b_rule",
					Name:       "My Rule B",
					Manual:     false,
					Success:    false,
					Missing:    false,
					Links:      []string{},
					Violations: []string{"violation"},
				},
			},
		},
	},
}

func TestNewClusterSummary(t *testing.T) {
	got, err := NewClusterSummary([]api.Report{fixtureReport1, fixtureReport2})
	if err != nil {
		t.Fatalf("NewClusterSummary raised an error %v", err)
	}

	want := api.ClusterSummary{
		Cluster: "exampleCluster",
		LatestReportSet: &api.ReportSet{
			Cluster:      "exampleCluster",
			Timestamp:    api.Time{},
			FailureCount: 4,
			SuccessCount: 1,
			Reports: []*api.ReportSummary{
				&api.ReportSummary{
					ID:           "fixtureReport1",
					Package:      "examplePackage.ID",
					Cluster:      "exampleCluster",
					Timestamp:    api.Time{},
					FailureCount: 2, // missing is a failure
					SuccessCount: 1,
				},
				&api.ReportSummary{
					ID:           "fixtureReport2",
					Package:      "examplePackage.ID",
					Cluster:      "exampleCluster",
					Timestamp:    api.Time{},
					FailureCount: 2,
					SuccessCount: 0,
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want;\ngot= %+v,\nwant=%+v", got, want)
	}
}

func TestNewReportSet(t *testing.T) {
	got, err := NewReportSet([]api.Report{fixtureReport1, fixtureReport2})
	if err != nil {
		t.Fatalf("NewReportSet raised an error %v", err)
	}

	want := api.ReportSet{
		Cluster:      "exampleCluster",
		Timestamp:    api.Time{},
		FailureCount: 4,
		SuccessCount: 1,
		Reports: []*api.ReportSummary{
			&api.ReportSummary{
				ID:           "fixtureReport1",
				Package:      "examplePackage.ID",
				Cluster:      "exampleCluster",
				Timestamp:    api.Time{},
				FailureCount: 2, // missing is a failure
				SuccessCount: 1,
			},
			&api.ReportSummary{
				ID:           "fixtureReport2",
				Package:      "examplePackage.ID",
				Cluster:      "exampleCluster",
				Timestamp:    api.Time{},
				FailureCount: 2,
				SuccessCount: 0,
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want;\ngot= %+v,\nwant=%+v", got, want)
	}
}

func TestNewReportSetDifferentClusters(t *testing.T) {
	exampleReportCluster1 := fixtureReport1
	exampleReportCluster1.Cluster = "exampleCluster1"
	exampleReportCluster2 := fixtureReport2
	exampleReportCluster2.Cluster = "exampleCluster2"

	_, got := NewReportSet([]api.Report{exampleReportCluster1, exampleReportCluster2})
	want := fmt.Errorf("reports must be for the same cluster")

	if got.Error() != want.Error() {
		t.Fatalf("got != want;\ngot= %s,\nwant=%s", got, want)
	}
}

func TestNewReportSetDifferentTimestamps(t *testing.T) {
	exampleReportTimestamp1 := fixtureReport1
	exampleReportTimestamp2 := fixtureReport2
	exampleReportTimestamp2.Timestamp = api.Time{Time: time.Now()}

	_, got := NewReportSet([]api.Report{exampleReportTimestamp1, exampleReportTimestamp2})
	want := fmt.Errorf("reports must have the same timestamp")

	if got.Error() != want.Error() {
		t.Fatalf("got != want;\ngot= %s,\nwant=%s", got, want)
	}
}

func TestReportSummarize(t *testing.T) {
	exampleReport := fixtureReport1
	exampleReport.ID = "exampleReport"

	got := exampleReport.Summarize()

	want := api.ReportSummary{
		ID:           "exampleReport",
		Package:      "examplePackage.ID",
		Cluster:      "exampleCluster",
		Timestamp:    api.Time{},
		FailureCount: 2, // missing is a failure
		SuccessCount: 1,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want;\ngot= %+v,\nwant=%+v", got, want)
	}
}

func TestNewReport(t *testing.T) {
	examplePackage := &packaging.PolicyManifest{
		SchemaVersion:  "0.1.0",
		ID:             "mypackage",
		Namespace:      "mynamespace",
		Name:           "My Package",
		RootQuery:      "data.pods",
		PackageVersion: "1.0.0",
		Sections: []packaging.Section{
			{
				ID:   "a_section",
				Name: "My section",
				Rules: []packaging.Rule{
					{
						ID:   "a_rule",
						Name: "My Rule A",
					},
					{
						ID:   "b_rule",
						Name: "My Rule B",
					},
					{
						ID:   "c_rule",
						Name: "My Rule C (missing)",
					},
				},
			},
		},
	}

	resultCollection := &results.ResultCollection{
		&results.Result{ID: rules.RuleToResult("a_rule"), Violations: []string{}},
		&results.Result{ID: rules.RuleToResult("b_rule"), Violations: []string{"violation"}},
	}

	got, err := NewReport(examplePackage, resultCollection)
	if err != nil {
		t.Fatalf("NewReport returned error: %v", err)
	}

	want := api.Report{
		PreflightVersion: version.PreflightVersion,
		Package:          examplePackage.ID,
		PackageInformation: api.PackageInformation{
			Namespace:     examplePackage.Namespace,
			ID:            examplePackage.ID,
			Version:       examplePackage.PackageVersion,
			SchemaVersion: examplePackage.SchemaVersion,
		},
		Name:        examplePackage.Name,
		Description: examplePackage.Description,
		Sections: []api.ReportSection{
			api.ReportSection{
				ID:   "a_section",
				Name: "My section",
				Rules: []api.ReportRule{
					api.ReportRule{
						ID:         "a_rule",
						Name:       "My Rule A",
						Manual:     false,
						Success:    true,
						Missing:    false,
						Links:      []string{},
						Violations: []string{},
					},
					api.ReportRule{
						ID:         "b_rule",
						Name:       "My Rule B",
						Manual:     false,
						Success:    false,
						Missing:    false,
						Links:      []string{},
						Violations: []string{"violation"},
					},
					api.ReportRule{
						ID:         "c_rule",
						Name:       "My Rule C (missing)",
						Manual:     false,
						Success:    false,
						Missing:    true,
						Links:      []string{},
						Violations: []string{},
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got != want; got=%+v, want=%+v", got, want)
	}
}
