package packaging

import "github.com/blang/semver"

// SupportsPreflightPrefix returns true if the SchemaVersion used supports rego rules with `preflight_` prefix over the IDs in the policy manifest. That behaviour was deprecated.
func (m *PolicyManifest) SupportsPreflightPrefix() (bool, error) {
	// If version is not defined, it is an old package.
	if m.SchemaVersion == "" {
		return true, nil
	}

	v010, err := semver.Make("0.1.0")
	if err != nil {
		panic(err)
	}

	v, err := semver.Make(m.SchemaVersion)
	if err != nil {
		return false, err
	}

	return v.LTE(v010), nil
}
