package rules

import (
	"fmt"
	"strings"
)

// RuleToResult takes a rule identifier and returns a result identifier
func RuleToResult(ruleID string) string {
	return strings.ReplaceAll(ruleID, ".", "_")
}

// ResultToRule rakes a result identifier and returns a rule identifier
func ResultToRule(resultID string) string {
	return strings.ReplaceAll(resultID, "_", ".")
}

// LegacyRuleToResult takes a legacy rule identifier and returns a result identifier
func LegacyRuleToResult(ruleID string) string {
	return fmt.Sprintf("preflight_%s", strings.ReplaceAll(ruleID, ".", "_"))
}
