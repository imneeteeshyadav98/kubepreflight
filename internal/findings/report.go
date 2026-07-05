package findings

import "time"

const SchemaVersion = "1.0"

type CoverageStatus string

const (
	CoverageComplete CoverageStatus = "complete"
	CoveragePartial  CoverageStatus = "partial"
	CoverageSkipped  CoverageStatus = "skipped"
)

type PlaneCoverage struct {
	Status CoverageStatus `json:"status"`
	Errors []string       `json:"errors,omitempty"`
}

type ScanCoverage struct {
	Kubernetes PlaneCoverage `json:"kubernetes"`
	AWS        PlaneCoverage `json:"aws"`
	Manifests  PlaneCoverage `json:"manifests"`
}

const CrossPlaneManifestAssumption = "Cross-plane matches assume supplied manifests target this cluster."

// Summary holds finding counts by severity for quick terminal/report headers.
type Summary struct {
	Blockers int `json:"blockers"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

// Report is the top-level findings.json document produced by a scan.
type Report struct {
	SchemaVersion      string       `json:"schemaVersion"`
	TargetVersion      string       `json:"targetVersion"`
	ClusterContext     string       `json:"clusterContext,omitempty"`
	Provider           string       `json:"provider,omitempty"` // "eks", or empty for a cluster-only scan
	ScannedAt          time.Time    `json:"scannedAt"`
	Assumptions        []string     `json:"assumptions,omitempty"`
	NamespaceAllowlist []string     `json:"namespaceAllowlist,omitempty"`
	Findings           []Finding    `json:"findings"`
	Summary            Summary      `json:"summary"`
	Coverage           ScanCoverage `json:"coverage"`
}

// NewReport builds a Report from a flat finding list, computing the summary.
func NewReport(targetVersion, clusterContext, provider string, scannedAt time.Time, fs []Finding) *Report {
	if fs == nil {
		fs = []Finding{}
	}
	r := &Report{
		SchemaVersion:  SchemaVersion,
		TargetVersion:  targetVersion,
		ClusterContext: clusterContext,
		Provider:       provider,
		ScannedAt:      scannedAt,
		Findings:       fs,
		Coverage: ScanCoverage{
			Kubernetes: PlaneCoverage{Status: CoverageComplete},
			AWS:        PlaneCoverage{Status: CoverageSkipped},
			Manifests:  PlaneCoverage{Status: CoverageSkipped},
		},
	}
	for _, f := range fs {
		if hasCrossPlaneMatch(f.Resources) && len(r.Assumptions) == 0 {
			r.Assumptions = []string{CrossPlaneManifestAssumption}
		}
		switch f.Severity {
		case SeverityBlocker:
			r.Summary.Blockers++
		case SeverityWarning:
			r.Summary.Warnings++
		case SeverityInfo:
			r.Summary.Infos++
		}
	}
	return r
}

func hasCrossPlaneMatch(refs []ResourceReference) bool {
	liveConcepts := map[string]bool{}
	for _, ref := range refs {
		if ref.Plane != PlaneLive {
			continue
		}
		if key, ok := ref.ConceptKey(); ok {
			liveConcepts[key] = true
		}
	}
	for _, ref := range refs {
		if ref.Plane != PlaneManifest {
			continue
		}
		if key, ok := ref.ConceptKey(); ok && liveConcepts[key] {
			return true
		}
	}
	return false
}

// resultAndExitCode is the single, shared priority order for the overall
// scan outcome. Result() and ExitCode() both derive from this so they can
// never disagree: incomplete coverage outranks blockers — a scan that
// couldn't collect all its evidence is never a fully-trusted "BLOCKED" or
// "CLEAN" result, even when real blockers were found with the evidence
// that WAS collected (those blockers stay fully visible in Summary/
// Findings; only the top-level result/exit code defer to "incomplete").
func (r *Report) resultAndExitCode() (string, int) {
	switch {
	case !r.IsComplete():
		return "INCOMPLETE", 3
	case r.Summary.Blockers > 0:
		return "BLOCKED", 2
	case r.Summary.Warnings > 0:
		return "PASSED_WITH_WARNINGS", 1
	default:
		return "CLEAN", 0
	}
}

// Result classifies the overall scan outcome from the finding summary.
func (r *Report) Result() string {
	result, _ := r.resultAndExitCode()
	return result
}

// ExitCode maps the scan outcome to the CLI exit-code contract documented
// in the README: 0 clean, 1 warnings only, 2 blockers present, 3
// incomplete coverage.
func (r *Report) ExitCode() int {
	_, code := r.resultAndExitCode()
	return code
}

func (r *Report) IsComplete() bool {
	return r.Coverage.Kubernetes.Status != CoveragePartial &&
		r.Coverage.AWS.Status != CoveragePartial &&
		r.Coverage.Manifests.Status != CoveragePartial
}
