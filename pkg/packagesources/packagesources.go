package packagesources

import (
	"context"
	"log"

	"github.com/jetstack/preflight/pkg/packagesources/local"
	"github.com/jetstack/preflight/pkg/packaging"
)

type PackageSource interface {
	// Load reads in all Preflight packages from a package source.
	Load() ([]*packaging.Package, error)
}

type PackageSourceConfig struct {
	Type string
	Dir  string
}

func NewPackageSources(ctx context.Context, config []*PackageSourceConfig) []PackageSource {
	packageSources := make([]PackageSource, 0)
	for _, packageSourceConfig := range config {
		if packageSourceConfig.Type == "local" {
			packageSource, err := local.NewLocalPackageSource(ctx, &local.LocalPackageSourceConfig{
				Path: packageSourceConfig.Dir,
			})
			if err != nil {
				log.Fatalf("%s", err)
			}
			packageSources = append(packageSources, packageSource)
		}
	}
	return packageSources
}
