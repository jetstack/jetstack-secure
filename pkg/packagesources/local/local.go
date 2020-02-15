package local

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jetstack/preflight/pkg/packaging"

	"gopkg.in/yaml.v2"
)

// ManifestName is the name of the file containing the Policy Manifest.
const ManifestName = "policy-manifest.yaml"

// Config is the configuration for the package source.
type Config struct {
	Dir string
}

func (c *Config) validate() error {
	if c.Dir == "" {
		return fmt.Errorf("invalid configuration: Dir is empty")
	}

	return nil
}

// NewPackageSource creates a new PackageSource from configuration.
// It validates the configuration.
func (c *Config) NewPackageSource() (*PackageSource, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	return &PackageSource{
		dir: c.Dir,
	}, nil
}

// PackageSource can load packages from a local directory.
type PackageSource struct {
	dir string
}

// Load loads packages from a local directory.
func (ps *PackageSource) Load() ([]*packaging.Package, error) {
	return loadPackagesFromDirectory(ps.dir)
}

// loadPackagesFromDirectory searches the directory specified for Preflight
// packages, recursively searching sub-directories. When it finds a package it
// loads it with loadPackageFromDirectory.
func loadPackagesFromDirectory(dirPath string) ([]*packaging.Package, error) {
	// Check we've been given a directory
	fi, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("Search path is not a directory")
	}

	// Create list to accumulate loaded packages
	var packages = make([]*packaging.Package, 0)

	// Check if a manifest file exists
	manifestPath := filepath.Join(dirPath, ManifestName)
	if _, err := os.Stat(manifestPath); err == nil {
		// If so this dir is a package
		loadedPackage, err := loadPackageFromDirectory(dirPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, loadedPackage)
		return packages, nil
	}

	// Otherwise assume this dir contains package dirs
	dirEntries, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range dirEntries {
		// Ignore things that aren't dirs
		if !entry.IsDir() {
			continue
		}
		// Allow nested package dirs
		entryPath := filepath.Join(dirPath, entry.Name())
		loadedPackages, err := loadPackagesFromDirectory(entryPath)
		// Any errors on nested dirs will stop search and return
		if err != nil {
			return nil, err
		}
		// Add newly loaded packages to main list
		packages = append(packages, loadedPackages...)
	}

	return packages, nil
}

// loadPackageFromDirectory loads a Preflight package from the directory
// specified.
func loadPackageFromDirectory(dirPath string) (*packaging.Package, error) {
	// Check we've been given a directory
	fi, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("Search path is not a directory")
	}

	// Parse manifest
	manifestPath := filepath.Join(dirPath, ManifestName)
	manifestBytes, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest packaging.PolicyManifest

	err = yaml.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return nil, err
	}
	log.Printf("Loaded Preflight Package %s: %s",
		manifest.GlobalID(),
		manifest.Name)

	// Look for `.rego` and `_test.rego` files and load them into a the corresponding map.
	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	regos := make(map[string]string)
	regoTests := make(map[string]string)
	for _, fi := range files {
		if IsPolicyFile(fi) {
			filePath := filepath.Join(dirPath, fi.Name())
			text, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			regos[fi.Name()] = string(text)
			log.Printf("Loaded policy file %s for package %s",
				fi.Name(),
				manifest.GlobalID())
		} else if IsPolicyTestFile(fi) {
			filePath := filepath.Join(dirPath, fi.Name())
			text, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			regoTests[fi.Name()] = string(text)
			log.Printf("Loaded test policy file %s for package %s",
				fi.Name(),
				manifest.GlobalID())
		}
	}

	// Create the struct and fire it back
	loadedPackage := &packaging.Package{
		PolicyManifest: &manifest,
		Rego:           regos,
		RegoTests:      regoTests,
	}
	return loadedPackage, nil
}

func IsPolicyFile(file os.FileInfo) bool {
	return !file.IsDir() &&
		strings.HasSuffix(file.Name(), ".rego") &&
		!strings.HasSuffix(file.Name(), "_test.rego")
}

func IsPolicyTestFile(file os.FileInfo) bool {
	return !file.IsDir() &&
		strings.HasSuffix(file.Name(), "_test.rego")
}
