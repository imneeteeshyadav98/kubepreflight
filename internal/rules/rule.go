// Package rules defines the check interface and registry every deterministic
// check (API-001, WH-001, etc.) registers against.
package rules

import (
	"kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// ScanContext bundles every collector's evidence for a single scan. AWS is
// nil whenever AWS enrichment wasn't attempted or was gracefully skipped
// (no credentials, or the provider isn't EKS) — rules that depend on it
// must check for nil and simply produce no findings, never error. This
// mirrors the deep dive's "five evidence planes feed one rules engine"
// architecture (Section 13.1): collectors stay independent, rules merge
// whatever evidence happens to be available.
type ScanContext struct {
	K8s *k8s.Snapshot
	AWS *aws.Snapshot
}

// Rule is a single deterministic check evaluated against a ScanContext for
// a given upgrade target version.
type Rule interface {
	// ID returns the rule's stable identifier, e.g. "API-001".
	ID() string
	// Evaluate inspects the scan context and returns zero or more findings.
	Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error)
}

// Registry holds the set of rules a scan will run.
type Registry struct {
	rules []Rule
}

// NewRegistry returns an empty registry. Rules are added via Register.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.rules = append(r.rules, rule)
}

// RunAll evaluates every registered rule against the scan context and
// returns the combined finding list. A rule error does not abort the
// others.
func (r *Registry) RunAll(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	var all []findings.Finding
	var firstErr error
	for _, rule := range r.rules {
		fs, err := rule.Evaluate(sc, targetVersion)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		all = append(all, fs...)
	}
	return all, firstErr
}
