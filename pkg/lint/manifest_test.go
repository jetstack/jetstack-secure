package lint

import (
	"strings"
	"testing"

	"github.com/jetstack/preflight/pkg/packaging"
)

var invalidRules = []struct {
	name         string
	rule         packaging.Rule
	expectedLint string
}{
	{
		name:         "Missing ID",
		rule:         packaging.Rule{},
		expectedLint: "Rule ID absent",
	},
	{
		name:         "Starting with a number",
		rule:         packaging.Rule{ID: "1_a"},
		expectedLint: "Malformed rule ID",
	},
	{
		name:         "Containing not allowed characters",
		rule:         packaging.Rule{ID: "a.1"},
		expectedLint: "Malformed rule ID",
	},
	{
		name:         "Rule Name absent",
		rule:         packaging.Rule{},
		expectedLint: "Rule Name absent",
	},
}

func TestLintRule(t *testing.T) {
	sectionID := "1.4"
	for _, ruleCase := range invalidRules {
		t.Run(
			ruleCase.name,
			func(t *testing.T) {
				lints := LintRule(sectionID, ruleCase.rule)
				var containsExpectedLint = false
				for _, l := range lints {
					if strings.HasPrefix(l.Lint, ruleCase.expectedLint) {
						containsExpectedLint = true
						break
					}
				}
				if !containsExpectedLint {
					t.Errorf(
						"Expected rule %#v to produce lint '%s', but it didn't. Got %v",
						ruleCase.rule,
						ruleCase.expectedLint,
						lints,
					)
				}
			})
	}
}

func TestLintRuleSuccess(t *testing.T) {
	validRule := packaging.Rule{
		ID:   "_1_B2_a3",
		Name: "My Rule",
	}
	sectionID := "1.2"
	lints := LintRule(sectionID, validRule)
	if len(lints) > 0 {
		lintStrings := make([]string, len(lints))
		for i, l := range lints {
			lintStrings[i] = l.Lint
		}
		t.Errorf("Failed on valid rule. Gave lints: %s", lintStrings)
	}
}

func TestLintSectionSuccess(t *testing.T) {
	validSection := packaging.Section{
		ID:   "_1_4",
		Name: "My section",
		Rules: []packaging.Rule{
			packaging.Rule{
				ID:   "_1_B2_a3",
				Name: "My Rule",
			},
		},
	}
	manifestID := "1"
	lints := LintSection(manifestID, validSection)
	if len(lints) > 0 {
		lintStrings := make([]string, len(lints))
		for i, l := range lints {
			lintStrings[i] = l.Lint
		}
		t.Errorf("Failed on valid section. Gave lints: %s", lintStrings)
	}
}

var invalidSections = []struct {
	name         string
	section      packaging.Section
	expectedLint string
}{
	{
		name:         "No rules",
		expectedLint: "No rules in section",
		section: packaging.Section{
			ID:   "1.4",
			Name: "My Section",
		},
	},
	{
		name:         "No Name",
		expectedLint: "Section Name absent",
		section: packaging.Section{
			ID: "1.4",
		},
	},
	{
		name:         "No ID",
		expectedLint: "Section ID absent",
		section: packaging.Section{
			Name: "My Section",
		},
	},
	{
		name:         "Starting with a number",
		expectedLint: "Malformed section ID",
		section:      packaging.Section{ID: "1_a"},
	},
	{
		name:         "Containing not allowed characters",
		expectedLint: "Malformed section ID",
		section:      packaging.Section{ID: "a.1"},
	},
	{
		name:         "Duplicate rule ID",
		expectedLint: "Rule ID 1.4.1 duplicated 2 times",
		section: packaging.Section{
			ID:   "1.4",
			Name: "My section",
			Rules: []packaging.Rule{
				packaging.Rule{
					ID:   "1.4.1",
					Name: "My Rule",
				},
				packaging.Rule{
					ID:   "1.4.1",
					Name: "My other Rule",
				},
			},
		},
	},
}

func TestSectionLint(t *testing.T) {
	packageID := "mypackage"
	for _, sectionCase := range invalidSections {
		t.Run(
			sectionCase.name,
			func(t *testing.T) {
				lints := LintSection(
					packageID,
					sectionCase.section,
				)
				var containsExpectedLint = false
				for _, l := range lints {
					if strings.HasPrefix(l.Lint, sectionCase.expectedLint) {
						containsExpectedLint = true
						break
					}
				}
				if !containsExpectedLint {
					lintNames := make([]string, len(lints))
					for i, lint := range lints {
						lintNames[i] = lint.Lint
					}
					t.Errorf(
						"Expected section %#v to produce lint '%s', but it didn't. Got %s",
						sectionCase.section,
						sectionCase.expectedLint,
						strings.Join(lintNames, ", "),
					)
				}
			},
		)
	}
}

func TestLintPolicyManifestSuccess(t *testing.T) {
	validPackage := packaging.PolicyManifest{
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
						Name: "My Rule",
					},
				},
			},
		},
	}
	lints := LintPolicyManifest(validPackage)
	if len(lints) > 0 {
		lintStrings := make([]string, len(lints))
		for i, l := range lints {
			lintStrings[i] = l.Lint
		}
		t.Errorf("Failed on valid package. Gave lints: %s", lintStrings)
	}
}

var invalidManifests = []struct {
	name         string
	manifest     packaging.PolicyManifest
	expectedLint string
}{
	{
		name:         "No sections",
		expectedLint: "No sections in manifest",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			ID:             "mypackage",
			Name:           "My Package",
			PackageVersion: "1.0.0",
		},
	},
	{
		name:         "No ID",
		expectedLint: "Manifest ID absent",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			Name:           "My Package",
			PackageVersion: "1.0.0",
		},
	},
	{
		name:         "No RootQuery",
		expectedLint: "Manifest RootQuery absent",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			Name:           "My Package",
			PackageVersion: "1.0.0",
		},
	},
	{
		name:         "No SchemaVersion",
		expectedLint: "Manifest SchemaVersion absent",
		manifest: packaging.PolicyManifest{
			ID:             "mypackage",
			Name:           "My Package",
			PackageVersion: "1.0.0",
		},
	},
	{
		name:         "SchemaVersion not semver",
		expectedLint: "Manifest SchemaVersion must be semver",
		manifest: packaging.PolicyManifest{
			ID:            "mypackage",
			Name:          "My Package",
			SchemaVersion: "1",
		},
	},
	{
		name:         "No PackageVersion",
		expectedLint: "Manifest PackageVersion absent",
		manifest: packaging.PolicyManifest{
			ID:            "mypackage",
			Name:          "My Package",
			SchemaVersion: "0.1.0",
		},
	},
	{
		name:         "PackageVersion not semver",
		expectedLint: "Manifest PackageVersion must be semver",
		manifest: packaging.PolicyManifest{
			ID:             "mypackage",
			Name:           "My Package",
			PackageVersion: "1",
		},
	},
	{
		name:         "Duplicated section ID",
		expectedLint: "Section ID 1.2 duplicated 2 times",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			ID:             "1",
			Name:           "My Package",
			PackageVersion: "1.0.0",
			Sections: []packaging.Section{
				{
					ID:   "1.2",
					Name: "My section",
					Rules: []packaging.Rule{
						{
							ID:   "1.2.3",
							Name: "My Rule",
						},
					},
				},
				{
					ID:   "1.2",
					Name: "My other section",
					Rules: []packaging.Rule{
						{
							ID:   "1.2.3",
							Name: "My other Rule",
						},
					},
				},
			},
		},
	},
	{
		name:         "Duplicated section Name",
		expectedLint: "Section Name 'foobar' duplicated 2 times",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			ID:             "1",
			Name:           "My Package",
			PackageVersion: "1.0.0",
			Sections: []packaging.Section{
				{
					ID:   "1.1",
					Name: "foobar",
					Rules: []packaging.Rule{
						{
							ID:   "1.1.3",
							Name: "My Rule",
						},
					},
				},
				{
					ID:   "1.2",
					Name: "foobar",
					Rules: []packaging.Rule{
						{
							ID:   "1.2.3",
							Name: "My other Rule",
						},
					},
				},
			},
		},
	},
	{
		name:         "Duplicated section Name",
		expectedLint: "Rule Name 'My Rule' duplicated 3 times",
		manifest: packaging.PolicyManifest{
			SchemaVersion:  "0.1.0",
			ID:             "1",
			Name:           "My Package",
			PackageVersion: "1.0.0",
			Sections: []packaging.Section{
				{
					ID:   "1.1",
					Name: "My section",
					Rules: []packaging.Rule{
						{
							ID:   "1.1.2",
							Name: "My Rule",
						},
						{
							ID:   "1.1.3",
							Name: "My Rule",
						},
					},
				},
				{
					ID:   "1.2",
					Name: "My other section",
					Rules: []packaging.Rule{
						{
							ID:   "1.2.3",
							Name: "My Rule",
						},
					},
				},
			},
		},
	},
}

func TestManfiestLint(t *testing.T) {
	for _, manifestCase := range invalidManifests {
		t.Run(
			manifestCase.name,
			func(t *testing.T) {
				lints := LintPolicyManifest(manifestCase.manifest)
				var containsExpectedLint = false
				for _, l := range lints {
					if l.Lint == manifestCase.expectedLint {
						containsExpectedLint = true
						break
					}
				}
				if !containsExpectedLint {
					lintNames := make([]string, len(lints))
					for i, lint := range lints {
						lintNames[i] = lint.Lint
					}
					t.Errorf(
						"Expected manifest %#v to produce lint '%s', but it didn't. Got %s",
						manifestCase.manifest,
						manifestCase.expectedLint,
						strings.Join(lintNames, ", "),
					)
				}
			},
		)
	}
}

func TestIsSemver(t *testing.T) {
	semverTcs := []struct {
		version string
		result  bool
	}{
		{"v1.0.0", true},
		{"1.0.0", true},
		{"v1.0.0-alpha", true},
		{"1.0.0-alpha", true},
		{"1", false},
		{"a", false},
		{"a.b.c", false},
		{"", false},
	}

	for _, tc := range semverTcs {
		t.Run(tc.version, func(t *testing.T) {
			if got, want := isSemver(tc.version), tc.result; got != want {
				t.Errorf("Failed to check if %s is semver: got = %v, want = %v", tc.version, got, want)
			}
		})
	}
}
