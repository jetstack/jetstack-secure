package rules

import "testing"

func TestRuleToResult(t *testing.T) {
	rule := "something_1.2.3"
	expectedResult := "something_1_2_3"
	if RuleToResult(rule) != expectedResult {
		t.Errorf(
			"Expected rule %q to render as result %q, but got %q",
			rule,
			expectedResult,
			RuleToResult(rule),
		)
	}
}

func TestLegacyRuleToResult(t *testing.T) {
	rule := "1.2.3"
	expectedResult := "preflight_1_2_3"
	if LegacyRuleToResult(rule) != expectedResult {
		t.Errorf(
			"Expected rule %q to render as result %q, but got %q",
			rule,
			expectedResult,
			RuleToResult(rule),
		)
	}
}

func TestResultToRule(t *testing.T) {
	result := "something_1_3_3"
	expectedRule := "something.1.3.3"
	if ResultToRule(result) != expectedRule {
		t.Errorf(
			"Expected result %q to render as rule %q, but got %q",
			result,
			expectedRule,
			ResultToRule(result),
		)
	}
}
