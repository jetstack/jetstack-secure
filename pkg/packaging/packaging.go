package packaging

import (
	"context"
	"fmt"

	"github.com/jetstack/preflight/pkg/results"
	"github.com/open-policy-agent/opa/rego"
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
	// RootQuery is the query needed in the REGO context to access the result of the checks.
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

// EvalPackage evaluated the rules in a package given an input
func EvalPackage(ctx context.Context, pkg Package, input interface{}) (*results.ResultCollection, error) {
	allResults := rego.ResultSet{}
	for file, text := range pkg.RegoText() {
		// Execute Open Policy Agent rules engine
		r := rego.New(
			rego.Query(pkg.PolicyManifest().RootQuery),
			rego.Module(file, text),
			rego.Input(input),
		)

		rs, err := r.Eval(ctx)
		if err != nil {
			return nil, fmt.Errorf("cannot evaluate rules in %q`: %v", file, err)
		}

		allResults = append(allResults, rs...)
	}

	rc, err := results.NewResultCollectionFromRegoResultSet(&allResults)
	if err != nil {
		return nil, fmt.Errorf("cannot read results from rego: %s", err)
	}

	return rc, nil
}
