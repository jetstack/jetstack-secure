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
	re := regexp.MustCompile(`^(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(section.ID)
	if len(matches) != 3 {
		lint("Malformed Section ID")
	} else {
		if matches[0] != section.ID {
			lint("Incorrectly formatted Section ID")
		}
		// No need to check matches[2], the regex implicitly did so
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
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(rule.ID)
	if len(matches) != 4 {
		lint("Malformed Rule ID")
	} else {
		if matches[0] != rule.ID {
			lint("Incorrectly formatted Rule ID")
		}
		if fmt.Sprintf("%s.%s", matches[1], matches[2]) != sectionID {
			lint("Rule ID not decendant of Section ID")
		}
		// No need to check matches[3], the regex implicitly did so
	}

	if rule.Name == "" {
		lint("Rule Name absent")
	}

	return lints
}
