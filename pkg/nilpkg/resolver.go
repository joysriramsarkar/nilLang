package nilpkg

import (
	"fmt"
	"strconv"
	"strings"
)

// Resolver handles dependency resolution
type Resolver struct {
	config   *Config
	database *Database
	registry *Registry
}

// NewResolver creates a new dependency resolver
func NewResolver(cfg *Config, db *Database, reg *Registry) *Resolver {
	return &Resolver{
		config:   cfg,
		database: db,
		registry: reg,
	}
}

// ResolveResult holds the result of dependency resolution
type ResolveResult struct {
	ToInstall []string // Packages that need to be installed
	ToUpdate  []string // Packages that need to be updated
	Satisfied []string // Packages already satisfied
	Conflicts []string // Version conflicts
}

// Resolve resolves dependencies for a package
func (r *Resolver) Resolve(dependencies map[string]string) (*ResolveResult, error) {
	result := &ResolveResult{
		ToInstall: []string{},
		ToUpdate:  []string{},
		Satisfied: []string{},
		Conflicts: []string{},
	}

	for depName, depVersion := range dependencies {
		if r.database.Has(depName) {
			installed, _ := r.database.Get(depName)
			if versionSatisfies(installed.Version, depVersion) {
				result.Satisfied = append(result.Satisfied, depName)
			} else {
				result.ToUpdate = append(result.ToUpdate, depName)
			}
		} else {
			// Check if available in registry
			if _, err := r.registry.Get(depName); err == nil {
				result.ToInstall = append(result.ToInstall, depName)
			} else {
				result.Conflicts = append(result.Conflicts,
					fmt.Sprintf("%s (not found in registry)", depName))
			}
		}
	}

	if len(result.Conflicts) > 0 {
		return result, fmt.Errorf("unresolvable dependencies: %v", result.Conflicts)
	}

	return result, nil
}

// versionSatisfies checks if installed version satisfies the requirement
func versionSatisfies(installed, required string) bool {
	// Simple version comparison
	// In production, use semantic versioning
	if required == "*" || required == "latest" {
		_, ok := parseVersion(installed)
		return ok
	}
	if required == installed {
		_, ok := parseVersion(installed)
		return ok
	}

	// Handle >= prefix
	if len(required) > 2 && required[:2] == ">=" {
		return compareVersions(installed, required[2:]) >= 0
	}

	// Handle ^ prefix (compatible with)
	if len(required) > 1 && required[0] == '^' {
		return compareMajorVersions(installed, required[1:])
	}

	return installed == required
}

func compareVersions(a, b string) int {
	// Simple version comparison (major.minor.patch)
	aParts, aOK := parseVersion(a)
	bParts, bOK := parseVersion(b)
	if !aOK || !bOK {
		return -1
	}

	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

func compareMajorVersions(a, b string) bool {
	aParts, aOK := parseVersion(a)
	bParts, bOK := parseVersion(b)
	if !aOK || !bOK {
		return false
	}
	return aParts[0] == bParts[0]
}

func parseVersion(v string) ([3]int, bool) {
	var parts [3]int
	segments := strings.Split(v, ".")
	if len(segments) != 3 {
		return parts, false
	}
	for i, segment := range segments {
		if segment == "" || (len(segment) > 1 && segment[0] == '0') {
			return parts, false
		}
		n, err := strconv.ParseUint(segment, 10, 31)
		if err != nil {
			return parts, false
		}
		parts[i] = int(n)
	}
	return parts, true
}
