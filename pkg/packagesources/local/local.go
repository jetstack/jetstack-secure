package local

import (
	"context"
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

const ManifestName = "policy-manifest.yaml"

type LocalPackageSourceConfig struct {
	Path string
}

func NewLocalPackageSource(ctx context.Context, config *LocalPackageSourceConfig) (*LocalPackageSource, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("Local package source has invalid path: %s", config.Path)
	}
	localPackageSource := &LocalPackageSource{
		path: config.Path,
	}
	return localPackageSource, nil
}

type LocalPackageSource struct {
	path string
}

func (ps *LocalPackageSource) Load() ([]*packaging.Package, error) {
	return loadPackagesFromDirectory(ps.path)
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
