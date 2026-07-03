package findings

import "time"

const CrossPlaneManifestAssumption = "Cross-plane matches assume supplied manifests target this cluster."

// Summary holds finding counts by severity for quick terminal/report headers.
type Summary struct {
	Blockers int `json:"blockers"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

// Report is the top-level findings.json document produced by a scan.
type Report struct {
	TargetVersion      string    `json:"targetVersion"`
	ClusterContext     string    `json:"clusterContext,omitempty"`
	Provider           string    `json:"provider,omitempty"` // "eks", or empty for a cluster-only scan
	ScannedAt          time.Time `json:"scannedAt"`
	Assumptions        []string  `json:"assumptions,omitempty"`
	NamespaceAllowlist []string  `json:"namespaceAllowlist,omitempty"`
	Findings           []Finding `json:"findings"`
	Summary            Summary   `json:"summary"`
}

// NewReport builds a Report from a flat finding list, computing the summary.
func NewReport(targetVersion, clusterContext, provider string, scannedAt time.Time, fs []Finding) *Report {
	if fs == nil {
		fs = []Finding{}
	}
	r := &Report{
		TargetVersion:  targetVersion,
		ClusterContext: clusterContext,
		Provider:       provider,
		ScannedAt:      scannedAt,
		Findings:       fs,
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

// Result classifies the overall scan outcome from the finding summary.
func (r *Report) Result() string {
	switch {
	case r.Summary.Blockers > 0:
		return "BLOCKED"
	case r.Summary.Warnings > 0:
		return "PASSED_WITH_WARNINGS"
	default:
		return "CLEAN"
	}
}

// ExitCode maps the scan outcome to the CLI exit-code contract documented
// in the README: 0 clean, 1 warnings only, 2 blockers present. CI
// integration lands in Week 6, but the contract is fixed now so it never
// has to change under existing scripts.
func (r *Report) ExitCode() int {
	switch {
	case r.Summary.Blockers > 0:
		return 2
	case r.Summary.Warnings > 0:
		return 1
	default:
		return 0
	}
}
