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
	Namespace     string    `yaml:"namespace"`
	ID            string    `yaml:"id"`
	DataGatherers []string  `yaml:"data-gatherers,omitempty"`
	RootQuery     string    `yaml:"root-query"`
	Name          string    `yaml:"name"`
	Description   string    `yaml:"description,omitempty"`
	Sections      []Section `yaml:"sections,omitempty"`
}

// GlobalID returns a global unique ID that contains the namespace and the ID.
func (m *PolicyManifest) GlobalID() string {
	return fmt.Sprintf("%s/%s", m.Namespace, m.ID)
}

// Section holds the information for a section of the policy manifest.
type Section struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Rules       []Rule `yaml:"rules,omitempty"`
}

// Rule holds the information for a rule.
type Rule struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Manual      bool     `yaml:"manual,omitempty"`
	Remediation string   `yaml:"remediation,omitempty"`
	Links       []string `yaml:"links,omitempty"`
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
