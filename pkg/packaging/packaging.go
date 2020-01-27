package packaging

import (
	"fmt"
)

// A Package is an interface that Package Sources should implement to
// provide details about a package in an agnostic way.
type Package interface {
	// PolicyManifest returns the manifest data about the policy
	// in struct format.
	PolicyManifest() *PolicyManifest

	// Return any rego files, with their names as the keys and
	// their content as strings.
	RegoText() map[string]string

	// Return any rego test files, with their names as the keys and
	// their content as strings.
	RegoTestsText() map[string]string
}

// PolicyManifest contains all the information about the policy manifest of the package.
type PolicyManifest struct {
	// SchemaVersion is the version of the PolicyManifest schema, and thus the version of the Preflight Package format. It follows semver.
	SchemaVersion string `yaml:"schema-version"`
	// PackageVersion is the version of the package. No format is enforced, but it is recommended to follow semver.
	PackageVersion string `yaml:"package-version"`
	// Namespace is the namespace of the package. We recommend to use FQDNs.
	Namespace string `yaml:"namespace"`
	// ID is the ID of the package itself.
	ID string `yaml:"id"`
	// DataGatherers is the list of data-gatherers the package depends on.
	DataGatherers []string `yaml:"data-gatherers,omitempty"`
	// RootQuery is the query needed in the Rego context to access the result of the checks.
	RootQuery string `yaml:"root-query"`
	// Name is the name of the package.
	Name string `yaml:"name"`
	// Description is a text describing the package.
	Description string `yaml:"description,omitempty"`
	// Sections contains the different sections inside the package.
	Sections []Section `yaml:"sections,omitempty"`
}

// GlobalID returns a global unique ID that contains the namespace and the ID.
func (m *PolicyManifest) GlobalID() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.ID)
}

// RuleIDs returns a list of the IDs of all the rules in this policy manifest
func (m *PolicyManifest) RuleIDs() []string {
	var ruleIDs []string
	for _, section := range m.Sections {
		for _, rule := range section.Rules {
			ruleIDs = append(ruleIDs, rule.ID)
		}
	}
	return ruleIDs
}

// Section holds the information for a section of the policy manifest.
type Section struct {
	// ID is the ID of the section.
	ID string `yaml:"id"`
	// Name is the name of the section.
	Name string `yaml:"name"`
	// Description is the description of the section.
	Description string `yaml:"description,omitempty"`
	// Rules contain all the rules in the section.
	Rules []Rule `yaml:"rules,omitempty"`
}

// Rule holds the information for a rule.
type Rule struct {
	// ID is the id of the rule.
	ID string `yaml:"id"`
	// Name is a shortname for the rule.
	Name string `yaml:"name"`
	// Description is a text describing what the rule is about.
	Description string `yaml:"description,omitempty"`
	// Manual indicated whether the rule can be evaluated automatically by Preflight or requires manual intervention.
	Manual bool `yaml:"manual,omitempty"`
	// Remediation is a text describing how to fix a failure of the rule.
	Remediation string `yaml:"remediation,omitempty"`
	// Links contains useful links related to the rule.
	Links []string `yaml:"links,omitempty"`
}
