// Package rules defines the check interface and registry every deterministic
// check (API-001, WH-001, etc.) registers against.
package rules

import (
	"fmt"

	"kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

// ScanContext bundles every collector's evidence for a single scan. AWS and
// Manifests are nil whenever that plane wasn't attempted or was gracefully
// skipped — rules that depend on them must check for nil and simply
// produce no findings, never error. This mirrors the deep dive's "five
// evidence planes feed one rules engine" architecture (Section 13.1):
// collectors stay independent, rules merge whatever evidence happens to be
// available.
type ScanContext struct {
	K8s       *k8s.Snapshot
	AWS       *aws.Snapshot
	Manifests *manifest.Snapshot
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

// RuleIDs returns the ID of every registered rule, in registration order.
// Used by internal/plan's tests to guard against a newly registered rule
// silently missing an explicit projection-policy decision.
func (r *Registry) RuleIDs() []string {
	ids := make([]string, len(r.rules))
	for i, rule := range r.rules {
		ids[i] = rule.ID()
	}
	return ids
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
		for i, f := range fs {
			if err := f.Validate(); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s finding %d: %w", rule.ID(), i, err)
				}
				continue
			}
			all = append(all, f)
		}
	}
	return all, firstErr
}
