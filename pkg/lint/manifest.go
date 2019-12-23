package lint

import (
	"fmt"
	"regexp"

	"github.com/jetstack/preflight/pkg/packaging"
)

// LintManifest lints the contents of a Policy Manifest
// that is already parsed.
func LintPolicyManifest(manifest packaging.PolicyManifest) []LintError {
	lints := make([]LintError, 0)

	lint := func(l string) {
		lints = append(lints, LintError{Lint: l})
	}

	if manifest.ID == "" {
		lint("Manifest ID absent")
	}

	if manifest.Namespace == "" {
		lint("Manifest Namespace absent")
	}

	if manifest.SchemaVersion == "" {
		lint("Manifest SchemaVersion absent")
	} else if !isSemver(manifest.SchemaVersion) {
		lint("Manifest SchemaVersion must be semver")
	}

	if manifest.PackageVersion == "" {
		lint("Manifest PackageVersion absent")
	} else if !isSemver(manifest.PackageVersion) {
		lint("Manifest PackageVersion must be semver")
	}

	sections := manifest.Sections
	if len(sections) == 0 {
		lint("No sections in manifest")
	} else {
		ids := make(map[string]int)
		names := make(map[string]int)
		for _, section := range sections {
			// Lint the section itself
			lints = append(lints, LintSection(manifest.ID, section)...)
			// Record the IDs and Names so we can do a duplicate search
			count, found := ids[section.ID]
			if found {
				ids[section.ID] = count + 1
			} else {
				ids[section.ID] = 1
			}

			count, found = names[section.Name]
			if found {
				names[section.Name] = count + 1
			} else {
				names[section.Name] = 1
			}
		}

		// Search for duplicate IDs
		for id, count := range ids {
			if count > 1 {
				lint(fmt.Sprintf("Section ID %s duplicated %d times", id, count))
			}
		}
		// Search for duplicate Names
		for name, count := range names {
			if count > 1 {
				lint(fmt.Sprintf("Section Name '%s' duplicated %d times", name, count))
			}
		}

		// Rule names are... harder. We need to check for
		// duplication across sections. We don't need to do this
		// for Rule IDs due to the inheritance requirments, so
		// other linting rules would fail if you duplicated a
		// rule ID across sections (because rule 1.2.3 can only
		// belong in section 1.2).

		ruleNames := make(map[string]int)
		for _, section := range sections {
			for _, rule := range section.Rules {
				count, found := ruleNames[rule.Name]
				if found {
					ruleNames[rule.Name] = count + 1
				} else {
					ruleNames[rule.Name] = 1
				}
			}
		}
		for ruleName, count := range ruleNames {
			if count > 1 {
				lint(fmt.Sprintf("Rule Name '%s' duplicated %d times", ruleName, count))
			}
		}
	}
	return lints
}

func LintSection(manifestID string, section packaging.Section) []LintError {
	lints := make([]LintError, 0)

	lint := func(l string) {
		lints = append(lints, LintError{Lint: l})
	}

	if section.ID == "" {
		lint("Section ID absent")
	}

	if section.Name == "" {
		lint("Section Name absent")
	}

	rules := section.Rules

	if len(rules) == 0 {
		lint("No rules in section")
	} else {
		ids := make(map[string]int)
		for _, rule := range rules {
			lints = append(lints, LintRule(section.ID, rule)...)

			// Record the IDs so we can do a duplicate search
			count, found := ids[rule.ID]
			if found {
				ids[rule.ID] = count + 1
			} else {
				ids[rule.ID] = 1
			}
		}

		// Search for duplicate IDs
		for id, count := range ids {
			if count > 1 {
				lint(fmt.Sprintf("Rule ID %s duplicated %d times", id, count))
			}
		}
	}
	return lints
}

func LintRule(sectionID string, rule packaging.Rule) []LintError {
	lints := make([]LintError, 0)

	lint := func(l string) {
		lints = append(lints, LintError{Lint: l})
	}

	if rule.ID == "" {
		lint("Rule ID absent")
	}

	if rule.Name == "" {
		lint("Rule Name absent")
	}

	return lints
}

func isSemver(version string) bool {
	match, _ := regexp.MatchString("^v?\\d+.\\d+.\\d+(-.+)?$", version)
	return match
}
