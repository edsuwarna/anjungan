package compliance

import "strings"

// ─── Prowler-Style Check Registry ─────────────────────────────────────────
//
// All checks are registered here and organized by profile (CIS Level 1 / 2)
// and category (ssh, kernel, fs, users, services, network, logging).
//
// Prowler-style features:
//   - Each check has a unique ID (e.g. "ssh_01", "kernel_03")
//   - Categorized by service area
//   - Metadata includes CIS reference, level, risk, and remediation
//   - Can run all checks, by level, by category, or individually

// CheckRegistry holds all registered compliance checks.
type CheckRegistry struct {
	items []CheckDefinition
	index map[string]int // check ID -> index in items
}

// NewCheckRegistry creates a registry with all default checks loaded.
func NewCheckRegistry() *CheckRegistry {
	r := &CheckRegistry{
		index: make(map[string]int),
	}
	r.Register(DefaultChecks()...)
	return r
}

// Register adds one or more checks to the registry.
func (r *CheckRegistry) Register(checks ...CheckDefinition) {
	for _, c := range checks {
		if _, exists := r.index[c.ID]; exists {
			// Overwrite existing
			r.index[c.ID] = len(r.items)
			r.items[r.index[c.ID]] = c
		} else {
			r.index[c.ID] = len(r.items)
			r.items = append(r.items, c)
		}
	}
}

// All returns all registered checks.
func (r *CheckRegistry) All() []CheckDefinition {
	out := make([]CheckDefinition, len(r.items))
	copy(out, r.items)
	return out
}

// Count returns the total number of registered checks.
func (r *CheckRegistry) Count() int {
	return len(r.items)
}

// GetByProfile returns checks matching the given profile.
// ProfileAll returns all checks.
// ProfileDocker returns checks with the "docker_" ID prefix.
// ProfileCISLevel1 and ProfileCISLevel2 exclude Docker checks — those only
// run under ProfileDocker since their CISLevel values collide.
func (r *CheckRegistry) GetByProfile(profile ScanProfile) []CheckDefinition {
	if profile == ProfileAll {
		return r.All()
	}

	// Docker checks have their own CIS benchmark levels (1 or 2) that don't
	// map to ScanProfile values — identify them by ID prefix instead.
	if profile == ProfileDocker {
		var out []CheckDefinition
		for _, c := range r.items {
			if strings.HasPrefix(c.ID, "docker_") {
				out = append(out, c)
			}
		}
		return out
	}

	var out []CheckDefinition
	for _, c := range r.items {
		// Exclude Docker checks from CIS Level 1/2 profiles — they have
		// overlapping CISLevel values (most are Level 1) that would cause
		// them to appear in Level 1/2 scans when they shouldn't.
		if strings.HasPrefix(c.ID, "docker_") {
			continue
		}
		if c.CISLevel == 0 || c.CISLevel == int(profile) {
			out = append(out, c)
		}
	}
	return out
}

// GetByCategory returns checks in the given category.
func (r *CheckRegistry) GetByCategory(category string) []CheckDefinition {
	var out []CheckDefinition
	for _, c := range r.items {
		if c.Category == category {
			out = append(out, c)
		}
	}
	return out
}

// GetByID returns a single check by its ID.
func (r *CheckRegistry) GetByID(id string) (CheckDefinition, bool) {
	idx, ok := r.index[id]
	if !ok {
		return CheckDefinition{}, false
	}
	return r.items[idx], true
}

// ListChecks returns public metadata for all registered checks.
func (r *CheckRegistry) ListChecks() []CheckInfo {
	var out []CheckInfo
	for _, c := range r.items {
		out = append(out, CheckInfo{
			ID:       c.ID,
			Category: c.Category,
			Title:    c.Title,
			Severity: c.Severity,
			CISID:    c.CISID,
			CISLevel: c.CISLevel,
			Risk:     c.Risk,
		})
	}
	return out
}

// Categories returns the unique list of category names.
func (r *CheckRegistry) Categories() []string {
	seen := make(map[string]bool)
	var cats []string
	for _, c := range r.items {
		if !seen[c.Category] {
			seen[c.Category] = true
			cats = append(cats, c.Category)
		}
	}
	return cats
}
