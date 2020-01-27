package exporter

import (
	"testing"
)

func TestRuleToResult(t *testing.T) {
	rule := "something_1.2.3"
	expectedResult := "something_1_2_3"
	if ruleToResult(rule) != expectedResult {
		t.Errorf(
			"Expected rule %q to render as result %q, but got %q",
			rule,
			expectedResult,
			ruleToResult(rule),
		)
	}
}

func TestLegacyRuleToResult(t *testing.T) {
	rule := "1.2.3"
	expectedResult := "preflight_1_2_3"
	if legacyRuleToResult(rule) != expectedResult {
		t.Errorf(
			"Expected rule %q to render as result %q, but got %q",
			rule,
			expectedResult,
			ruleToResult(rule),
		)
	}
}

func TestResultToRule(t *testing.T) {
	result := "something_1_3_3"
	expectedRule := "something.1.3.3"
	if resultToRule(result) != expectedRule {
		t.Errorf(
			"Expected result %q to render as rule %q, but got %q",
			result,
			expectedRule,
			resultToRule(result),
		)
	}
}
