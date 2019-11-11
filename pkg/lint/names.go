package lint

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/jetstack/preflight/pkg/packaging"
)

type RuleName struct {
	Package int
	Section int
	Rule    int
}

func (rn RuleName) String() string {
	return fmt.Sprintf("%d.%d.%d", rn.Package, rn.Section, rn.Rule)
}

// Should be a const but Go doesn't support that.
var ruleNameRegoRegex = regexp.MustCompile(`^preflight_(\d+)_(\d+)_(\d+)$`)

func NewRuleNameFromRego(n string) (*RuleName, error) {
	matches := ruleNameRegoRegex.FindStringSubmatch(n)
	if len(matches) == 4 {
		// These can't error, the regex already confirms that they're integers
		p, _ := strconv.Atoi(matches[1])
		s, _ := strconv.Atoi(matches[2])
		r, _ := strconv.Atoi(matches[3])
		return &RuleName{p, s, r}, nil
	} else {
		return nil, errors.New("Not a valid preflight rule name")
	}
}

var ruleNameManifestRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

func NewRuleNameFromManifest(n string) (*RuleName, error) {
	matches := ruleNameManifestRegex.FindStringSubmatch(n)
	if len(matches) == 4 {
		// These can't error, the regex already confirms that they're integers
		p, _ := strconv.Atoi(matches[1])
		s, _ := strconv.Atoi(matches[2])
		r, _ := strconv.Atoi(matches[3])
		return &RuleName{p, s, r}, nil
	} else {
		return nil, errors.New("Not a valid preflight rule name")
	}
}

func CollectManifestRuleNames(manifest packaging.PolicyManifest) []RuleName {
	ruleNames := make([]RuleName, 0)
	for _, section := range manifest.Sections {
		for _, rule := range section.Rules {
			if !rule.Manual {
				rn, err := NewRuleNameFromManifest(rule.ID)
				if err == nil {
					ruleNames = append(ruleNames, *rn)
				}
			}
		}
	}
	return ruleNames
}
