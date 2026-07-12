// Package comparison diffs two findings.json documents (a baseline and a
// current scan of the same cluster/target) into new/resolved/changed/
// unchanged findings plus verdict and readiness-score movement. It has no
// CLI dependency so the Console, the GitHub Action, and any future API can
// reuse the same engine internal/cli/compare.go wires up.
package comparison

import "kubepreflight/internal/findings"

// SchemaVersion identifies the comparison document format, matching the
// URN-style versioning internal/plan.ActionPlanSchemaVersion already
// established for kubepreflight's other standalone JSON documents (not
// Kubernetes's own "apiVersion" convention, which this project doesn't use
// anywhere else).
const SchemaVersion = "kubepreflight.io/scan-comparison/v1"

// Comparison is the top-level output document.
type Comparison struct {
	SchemaVersion string    `json:"schemaVersion"`
	Warnings      []string  `json:"warnings,omitempty"`
	Summary       Summary   `json:"summary"`
	New           []Entry   `json:"new"`
	Resolved      []Entry   `json:"resolved"`
	Changed       []Changed `json:"changed"`
	Unchanged     []Entry   `json:"unchanged"`
}

// Summary is the at-a-glance verdict/score movement and counts.
type Summary struct {
	BaselineVerdict        string `json:"baselineVerdict"`
	CurrentVerdict         string `json:"currentVerdict"`
	VerdictChanged         bool   `json:"verdictChanged"`
	BaselineReadinessScore int    `json:"baselineReadinessScore"`
	CurrentReadinessScore  int    `json:"currentReadinessScore"`
	ReadinessScoreDelta    int    `json:"readinessScoreDelta"`
	New                    int    `json:"new"`
	Resolved               int    `json:"resolved"`
	Changed                int    `json:"changed"`
	Unchanged              int    `json:"unchanged"`
	NewBlockers            int    `json:"newBlockers"`
	ResolvedBlockers       int    `json:"resolvedBlockers"`
}

// Entry wraps one finding in the New/Resolved/Unchanged buckets. It's the
// full finding, not a summary — a comparison consumer (Console, change
// ticket) needs the same evidence/remediation a plain scan would show.
type Entry struct {
	findings.Finding
}

// Changed is one finding present in both scans (same fingerprint) with at
// least one tracked field different between them. Tracked fields are:
// severity, priority, confidence, canUpgradeContinue, affectedScope,
// ruleId, and resource identity (the last two are defensive -- Fingerprint
// itself already hashes ruleID and each resource's concept key, so two
// findings genuinely differing in either could never share a fingerprint
// in practice, but the check costs nothing and stays correct if that ever
// changes). Evidence/remediation text is deliberately NOT tracked -- see
// compare.go's diffFinding -- so a copy-edit to a remediation string never
// shows up as a "Changed" finding, only real decision-relevant movement.
type Changed struct {
	Fingerprint string                       `json:"fingerprint"`
	RuleID      string                       `json:"ruleId"`
	Resources   []findings.ResourceReference `json:"resources"`
	Changes     map[string]FieldChange       `json:"changes"`
}

// FieldChange is one tracked field's before/after value, always rendered
// as strings so severity/priority/bool/scope changes share one shape.
type FieldChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}
