package redact

import (
	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
)

// Report redacts every AWS ARN and EC2-style internal node hostname
// reachable from r, in place. Nil-safe.
func Report(r *findings.Report) {
	if r == nil {
		return
	}
	r.ClusterContext = Text(r.ClusterContext)
	r.Coverage.Kubernetes.Errors = Strings(r.Coverage.Kubernetes.Errors)
	r.Coverage.AWS.Errors = Strings(r.Coverage.AWS.Errors)
	r.Coverage.Manifests.Errors = Strings(r.Coverage.Manifests.Errors)

	if r.EKSCluster != nil {
		r.EKSCluster.ARN = Text(r.EKSCluster.ARN)
	}
	redactAPICompatibilitySummary(r.APICompatibility)
	for i := range r.EKSNodegroups {
		ng := &r.EKSNodegroups[i]
		ng.AutoScalingGroups = Strings(ng.AutoScalingGroups)
		for j := range ng.HealthIssues {
			ng.HealthIssues[j].Message = Text(ng.HealthIssues[j].Message)
			ng.HealthIssues[j].ResourceIDs = Strings(ng.HealthIssues[j].ResourceIDs)
		}
	}
	for i := range r.EKSUpgradeInsights {
		in := &r.EKSUpgradeInsights[i]
		in.Description = Text(in.Description)
		in.Recommendation = Text(in.Recommendation)
		in.DeprecationDetails = Strings(in.DeprecationDetails)
		in.AddonCompatibility = Strings(in.AddonCompatibility)
		StringMapValues(in.AdditionalInfo)
	}

	for i := range r.Findings {
		redactFinding(&r.Findings[i])
	}
}

func redactAPICompatibilitySummary(s *findings.APICompatibilitySummary) {
	if s == nil {
		return
	}
	for i := range s.RemovedFamilies {
		s.RemovedFamilies[i].Resources = Strings(s.RemovedFamilies[i].Resources)
	}
	for i := range s.DeprecatedFamilies {
		s.DeprecatedFamilies[i].Resources = Strings(s.DeprecatedFamilies[i].Resources)
	}
}

func redactFinding(f *findings.Finding) {
	f.Message = Text(f.Message)
	f.Evidence = Strings(f.Evidence)
	f.Remediation = Text(f.Remediation)
	for i := range f.Resources {
		res := &f.Resources[i]
		res.Name = Text(res.Name)
		res.ProviderID = Text(res.ProviderID)
		res.ProviderName = Text(res.ProviderName)
	}
	if f.RemediationDetail == nil {
		return
	}
	d := f.RemediationDetail
	d.Diff = Text(d.Diff)
	d.VerifyCommand = Text(d.VerifyCommand)
	d.ExpectedResult = Text(d.ExpectedResult)
	for i := range d.Changes {
		d.Changes[i].Current = Text(d.Changes[i].Current)
		d.Changes[i].Required = Text(d.Changes[i].Required)
	}
	redactRemediationAction(d.SafeFix)
	redactRemediationAction(d.Emergency)
	redactRemediationAction(d.BreakGlass)
}

func redactRemediationAction(a *findings.RemediationAction) {
	if a == nil {
		return
	}
	a.Command = Text(a.Command)
	a.Steps = Strings(a.Steps)
}

// Comparison redacts every AWS ARN and EC2-style internal node hostname
// reachable from c, in place. Nil-safe.
//
// New/Resolved/Unchanged entries embed the full findings.Finding (see
// comparison.Entry's own comment: "the full finding, not a summary"), so
// they carry the same leak surface as Report — this exists specifically
// because comparing two *unredacted* findings.json files (the common case:
// `kubepreflight scan` without --redact-sensitive-identifiers, then
// `kubepreflight compare` on the results) would otherwise silently forward
// every sensitive value straight into comparison.json even when the
// operator's actual intent, by passing this flag on the compare command
// itself, was to share the comparison output specifically.
func Comparison(c *comparison.Comparison) {
	if c == nil {
		return
	}
	c.Warnings = Strings(c.Warnings)
	for i := range c.New {
		redactFinding(&c.New[i].Finding)
	}
	for i := range c.Resolved {
		redactFinding(&c.Resolved[i].Finding)
	}
	for i := range c.Unchanged {
		redactFinding(&c.Unchanged[i].Finding)
	}
	for i := range c.Changed {
		for j := range c.Changed[i].Resources {
			res := &c.Changed[i].Resources[j]
			res.Name = Text(res.Name)
			res.ProviderID = Text(res.ProviderID)
			res.ProviderName = Text(res.ProviderName)
		}
	}
}

// RollbackAssessment redacts every AWS ARN and EC2-style internal node
// hostname reachable from a, in place. Nil-safe. Cluster.Name/Region are
// deliberately left alone — consistent with the rest of the product (see
// the real EKS case-study evidence redaction) treating a cluster name and
// region as non-sensitive; only the ARN and node hostnames are redacted.
func RollbackAssessment(a *rollback.Assessment) {
	if a == nil {
		return
	}
	for i := range a.Checks {
		a.Checks[i].Evidence = Strings(a.Checks[i].Evidence)
	}
}

// PlanReport redacts every AWS ARN and EC2-style internal node hostname
// reachable from pr, in place, including every hop's *findings.Report
// (hop 1 and every predicted future hop share this same redaction, so a
// multi-hop upgrade-plan.json is never redacted for the immediate hop but
// not the rest) and the derived UpgradeActionPlan, whose Commands/Reason
// fields are built from hop 1's findings and can carry the same identifiers
// forward into free text even after the source finding is redacted.
func PlanReport(pr *plan.PlanReport) {
	if pr == nil {
		return
	}
	pr.ClusterContext = Text(pr.ClusterContext)
	for i := range pr.Hops {
		Report(pr.Hops[i].Report)
	}
	if pr.ActionPlan == nil {
		return
	}
	for i := range pr.ActionPlan.Phases {
		phase := &pr.ActionPlan.Phases[i]
		phase.Description = Text(phase.Description)
		phase.Gate = Text(phase.Gate)
		for j := range phase.Actions {
			action := &phase.Actions[j]
			action.Title = Text(action.Title)
			action.Reason = Text(action.Reason)
			action.SuccessCriteria = Strings(action.SuccessCriteria)
			action.Commands = Strings(action.Commands)
		}
	}
}
