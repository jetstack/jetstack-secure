package lint

import (
	"fmt"
	"testing"
)

func TestRuleNameString(t *testing.T) {
	rn := RuleName{
		Package: 1,
		Section: 2,
		Rule:    3,
	}
	stringRn := fmt.Sprintf("%s", rn)
	if stringRn != "1.2.3" {
		t.Errorf("Expected a RuleName to have a string representation of \"1.2.3\", got \"%s\"", stringRn)
	}
}

func TestNewRuleNameFromRego(t *testing.T) {
	regoRuleName := "preflight_1_30_3"
	rn, err := NewRuleNameFromRego(regoRuleName)
	if err != nil {
		t.Error("Failed to parse rego rule name", err)
	}
	expectedRuleName := RuleName{
		Package: 1,
		Section: 30,
		Rule:    3,
	}
	if expectedRuleName != *rn {
		t.Errorf("Parsed rule name incorrectly, expected %s got %s", expectedRuleName, rn)
	}
}

var invalidRegoInputs = []string{
	"_1_2_3",
	"preflight_1_a_3",
	"preflight_1_2",
	"preflight_1_-2_3",
	"preflight_1_2_3_4",
	"foo_preflight_1_2_3",
}

func TestInvalidRegoRuleName(t *testing.T) {
	for _, in := range invalidRegoInputs {
		t.Run(in, func(t *testing.T) {
			_, err := NewRuleNameFromRego(in)
			if err == nil {
				t.Errorf("Failed to error when parsing invalid rule name %s", in)
			}
		})
	}
}

func TestNewRuleNameFromManifest(t *testing.T) {
	regoRuleName := "1.30.3"
	rn, err := NewRuleNameFromManifest(regoRuleName)
	if err != nil {
		t.Error("Failed to parse manifest rule name", err)
	}
	expectedRuleName := RuleName{
		Package: 1,
		Section: 30,
		Rule:    3,
	}
	if expectedRuleName != *rn {
		t.Errorf("Parsed rule name incorrectly, expected %s got %s", expectedRuleName, rn)
	}
}

var invalidManifestInputs = []string{
	"1.2..3",
	"1.-2.3",
	"1.2",
	"1.2.3.4",
	"foo.1.2.3",
}

func TestInvalidManifestRuleName(t *testing.T) {
	for _, in := range invalidManifestInputs {
		t.Run(in, func(t *testing.T) {
			_, err := NewRuleNameFromManifest(in)
			if err == nil {
				t.Errorf("Failed to error when parsing invalid rule name %s", in)
			}
		})
	}
}
