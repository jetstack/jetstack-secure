package packagesources

import (
	"fmt"
	"log"

	"github.com/jetstack/preflight/pkg/packaging"
)

// TypedConfig wraps a PackageSource config adding a field that identifies its type.
type TypedConfig struct {
	Type   string
	Config Config
}

// NewPackageSource construct a PackageSource from a TypedConfig.
func (tc *TypedConfig) NewPackageSource() (PackageSource, error) {
	switch tc.Type {
	case "local":
		ps, err := tc.Config.NewPackageSource()
		if err != nil {
			log.Fatalf("%s", err)
		}
		return ps, nil
	default:
		return nil, fmt.Errorf("invalid package source type %q", tc.Type)
	}
}

// Config is the configuration of a PackageSource. Acts as a factory for PackageSource.
type Config interface {
	// NewPackageSource constructs a PackageSource based on the configuration.
	NewPackageSource() (PackageSource, error)
}

// PackageSource can load packages.
type PackageSource interface {
	// Load reads in all Preflight packages from a package source.
	Load() ([]*packaging.Package, error)
}
