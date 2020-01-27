package reports

import (
	"reflect"
	"testing"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/packaging"
	"github.com/jetstack/preflight/pkg/results"
	"github.com/jetstack/preflight/pkg/rules"
	"github.com/jetstack/preflight/pkg/version"
)

func TestConstructReport(t *testing.T) {
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

	got, err := ConstructReport(examplePackage, resultCollection)
	if err != nil {
		t.Fatalf("ConstructReport returned error: %v", err)
	}

	want := api.Report{
		PreflightVersion: version.PreflightVersion,
		Package:          examplePackage.ID,
		PackageInformation: api.PackageInformation{
			Namespace: examplePackage.Namespace,
			ID:        examplePackage.ID,
			Version:   examplePackage.PackageVersion,
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
