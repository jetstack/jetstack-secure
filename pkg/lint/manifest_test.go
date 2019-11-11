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
		name:         "Non-numeric rule ID",
		rule:         packaging.Rule{ID: "1.4.c"},
		expectedLint: "Malformed Rule ID",
	},
	{
		name:         "Rule ID too short",
		rule:         packaging.Rule{ID: "1.4"},
		expectedLint: "Malformed Rule ID",
	},
	{
		name:         "Rule ID too long",
		rule:         packaging.Rule{ID: "1.4.3.6"},
		expectedLint: "Malformed Rule ID",
	},
	{
		name:         "Rule ID not decendant",
		rule:         packaging.Rule{ID: "1.5.1"},
		expectedLint: "Rule ID not decendant of Section ID",
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
					if l.Lint == ruleCase.expectedLint {
						containsExpectedLint = true
						break
					}
				}
				if !containsExpectedLint {
					t.Errorf(
						"Expected rule %#v to produce lint '%s', but it didn't.",
						ruleCase.rule,
						ruleCase.expectedLint,
					)
				}
			})
	}
}

func TestLintRuleSuccess(t *testing.T) {
	validRule := packaging.Rule{
		ID:   "1.2.3",
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
		ID:   "1.4",
		Name: "My section",
		Rules: []packaging.Rule{
			packaging.Rule{
				ID:   "1.4.1",
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
		name:         "Non-numeric ID",
		expectedLint: "Malformed Section ID",
		section: packaging.Section{
			ID: "1.a",
		},
	},
	{
		name:         "Section ID too short",
		expectedLint: "Malformed Section ID",
		section: packaging.Section{
			ID: "1",
		},
	},
	{
		name:         "Section ID too long",
		expectedLint: "Malformed Section ID",
		section: packaging.Section{
			ID: "1.4.3",
		},
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
					if l.Lint == sectionCase.expectedLint {
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
		ID:        "mypackage",
		Namespace: "mynamespace",
		Name:      "My Package",
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
			ID:   "mypackage",
			Name: "My Package",
		},
	},
	{
		name:         "No ID",
		expectedLint: "Manifest ID absent",
		manifest: packaging.PolicyManifest{
			Name: "My Package",
		},
	},
	{
		name:         "Duplicated section ID",
		expectedLint: "Section ID 1.2 duplicated 2 times",
		manifest: packaging.PolicyManifest{
			ID:   "1",
			Name: "My Package",
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
			ID:   "1",
			Name: "My Package",
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
			ID:   "1",
			Name: "My Package",
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
