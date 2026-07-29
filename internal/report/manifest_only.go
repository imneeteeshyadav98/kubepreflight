package report

import "github.com/imneeteeshyadav98/kubepreflight/internal/findings"

func manifestOnlyCleanNotice(r *findings.Report) string {
	if r == nil || r.Result() != "CLEAN" || !isManifestOnlyReport(r) {
		return ""
	}
	return "Manifest API checks clean. Cluster, AWS, scheduling, disruption, add-on, node, CRD, and webhook checks were not evaluated in manifest-only mode."
}

func isManifestOnlyReport(r *findings.Report) bool {
	return r.Coverage.Manifests.Status != findings.CoverageSkipped &&
		r.Coverage.Kubernetes.Status == findings.CoverageSkipped &&
		r.Coverage.AWS.Status == findings.CoverageSkipped
}
