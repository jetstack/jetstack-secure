package local

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jetstack/preflight/pkg/packaging"

	"gopkg.in/yaml.v2"
)

const ManifestName = "policy-manifest.yaml"

type LocalPackage struct {
	manifest   *packaging.PolicyManifest
	rules      map[string]string
	rulesTests map[string]string
}

func (lp *LocalPackage) PolicyManifest() *packaging.PolicyManifest {
	return lp.manifest
}

func (lp *LocalPackage) RegoText() map[string]string {
	return lp.rules
}

func (lp *LocalPackage) RegoTestsText() map[string]string {
	return lp.rulesTests
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

// LoadLocalPackage eagerly reads and parses files on disk to present
// a Preflight package.
func LoadLocalPackage(dirPath string) (*LocalPackage, error) {
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
	return &LocalPackage{manifest: &manifest, rules: regos, rulesTests: regoTests}, nil
}

func LoadLocalPackages(dirPath string) ([]*LocalPackage, error) {
	// Check we've been given a directory
	fi, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, errors.New("Search path is not a directory")
	}

	// Create list to accumulate loaded packages
	var packages = make([]*LocalPackage, 0)

	// Check if a manifest file exists
	manifestPath := filepath.Join(dirPath, ManifestName)
	if _, err := os.Stat(manifestPath); err == nil {
		// If so this dir is a package
		loadedPackage, err := LoadLocalPackage(dirPath)
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
		loadedPackages, err := LoadLocalPackages(entryPath)
		// Any errors on nested dirs will stop search and return
		if err != nil {
			return nil, err
		}
		// Add newly loaded packages to main list
		packages = append(packages, loadedPackages...)
	}

	return packages, nil
}
