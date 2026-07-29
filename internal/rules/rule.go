// Package rules defines the check interface and registry every deterministic
// check (API-001, WH-001, etc.) registers against.
package rules

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// ScanContext bundles every collector's evidence for a single scan. AWS and
// Manifests are nil whenever that plane wasn't attempted or was gracefully
// skipped — rules that depend on them must check for nil and simply
// produce no findings, never error. This mirrors the deep dive's "five
// evidence planes feed one rules engine" architecture (Section 13.1):
// collectors stay independent, rules merge whatever evidence happens to be
// available.
type ScanContext struct {
	K8s                 *k8s.Snapshot
	AWS                 *aws.Snapshot
	Manifests           *manifest.Snapshot
	UpgradeContext      findings.UpgradeContext
	KubernetesRequested bool
	AWSRequested        bool
	ManifestsRequested  bool
}

// Rule is a single deterministic check evaluated against a ScanContext for
// a given upgrade target version.
type Rule interface {
	// ID returns the rule's stable identifier, e.g. "API-001".
	ID() string
	// Evaluate inspects the scan context and returns zero or more findings.
	Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error)
}

// DependencyRule is implemented by rules that declare the evidence they
// require to make "no finding" meaningful. The registry evaluates these
// dependencies uniformly instead of mirroring rule-specific error keys in a
// fragile central list.
type DependencyRule interface {
	Rule
	EvidenceDependencies() []EvidenceDependency
}

// ContextDependencyRule lets a rule declare dependencies that are only
// required after already-available evidence proves they are relevant. For
// example, Service/EndpointSlice evidence is required for WH-002 only when
// a webhook with clientConfig.service exists; an empty webhook list is a
// clean evaluated result, not insufficient evidence because Service listing
// failed.
type ContextDependencyRule interface {
	Rule
	EvidenceDependenciesFor(*ScanContext) []EvidenceDependency
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
//
// KNOWN, INTENTIONALLY UNFIXED LIMITATION: only the very first rule (in
// registration order) to return an error has that error retained as
// firstErr -- every subsequent rule's error is silently discarded, its only
// trace being that its findings are dropped too. This is a confirmed,
// pre-existing gap (docs/roadmap/v1.3.0-scope-audit.md, "Confirmed product
// gaps" #9 and Decision 5) that v1.3.0 deliberately does NOT fix here: the
// fix (accumulate every rule's error, e.g. via errors.Join) is scoped as an
// independent, small v1.2.2 patch candidate, orthogonal to the
// RuleExecutionRecord model RunAllWithExecutions adds below. Do not "fix"
// this by changing what RunAll returns -- doing so here would be exactly
// the out-of-scope change the locked release document forbids for this PR.
func (r *Registry) RunAll(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	all, firstErr, _ := r.runAll(sc, targetVersion)
	return all, firstErr
}

// runAll is the shared core of RunAll and RunAllWithExecutions. It
// preserves RunAll's existing first-error-only behavior byte-for-byte (same
// loop, same conditions, same values) -- this refactor changes nothing
// about what RunAll itself returns. It additionally tracks which single
// rule ID the surviving firstErr came from (firstErrRuleID), which
// RunAllWithExecutions needs so it can attribute State: failed to the one
// rule RunAll's existing logic actually preserved an error for, rather than
// guessing or over-attributing it to every rule that happened to error.
func (r *Registry) runAll(sc *ScanContext, targetVersion string) (all []findings.Finding, firstErr error, firstErrRuleID string) {
	for _, rule := range r.rules {
		fs, err := rule.Evaluate(sc, targetVersion)
		if err != nil && firstErr == nil {
			firstErr = err
			firstErrRuleID = rule.ID()
		}
		for i, f := range fs {
			if err := f.Validate(); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s finding %d: %w", rule.ID(), i, err)
					firstErrRuleID = rule.ID()
				}
				continue
			}
			all = append(all, f)
		}
	}
	return all, firstErr, firstErrRuleID
}

// RunAllWithExecutions runs every rule this Registry contains and returns
// one findings.RuleExecutionRecord per rule ID in rules.AllRuleIDs(), the
// full rule universe this build knows about. Unlike RunAll, every rule is
// evaluated independently: a rule error, panic, invalid finding, or
// insufficient collector evidence is captured on that rule's execution
// record and does not prevent later rules from running.
//
// The returned error is errors.Join over every rule-level failure for
// programmatic callers. CLI report paths must still use the returned
// findings and RuleExecutions so users receive an incomplete report instead
// of a generic abort.
func (r *Registry) RunAllWithExecutions(sc *ScanContext, targetVersion string) ([]findings.Finding, []findings.RuleExecutionRecord, error) {
	invoked := make(map[string]bool, len(r.rules))
	ruleByID := make(map[string]Rule, len(r.rules))
	for _, rule := range r.rules {
		invoked[rule.ID()] = true
		ruleByID[rule.ID()] = rule
	}

	universe := AllRuleIDs()
	var all []findings.Finding
	var errs []error
	records := make([]findings.RuleExecutionRecord, 0, len(universe))
	for _, ruleID := range universe {
		if !invoked[ruleID] {
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityNotApplicable,
				State:         findings.ExecutionNotEvaluated,
				Reason:        "not registered for this scan mode",
			})
			continue
		}

		rule := ruleByID[ruleID]
		depStatus := evaluateRuleDependencies(rule, sc)
		if depStatus.notApplicable {
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityNotApplicable,
				State:         findings.ExecutionNotEvaluated,
				Reason:        depStatus.reason,
			})
			continue
		}
		if depStatus.missingPlane {
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionInsufficientEvidence,
				Reason:        depStatus.reason,
			})
			continue
		}

		fs, err := evaluateRuleSafely(rule, sc, targetVersion)
		var ruleErrs []error
		for i, f := range fs {
			if err := f.Validate(); err != nil {
				ruleErrs = append(ruleErrs, fmt.Errorf("finding %d: %w", i, err))
				continue
			}
			all = append(all, f)
		}
		if err != nil {
			ruleErrs = append(ruleErrs, err)
		}
		if len(ruleErrs) > 0 {
			joined := errors.Join(ruleErrs...)
			errs = append(errs, fmt.Errorf("%s: %w", ruleID, joined))
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionFailed,
				Reason:        sanitizeRuleError(joined),
			})
			continue
		}
		if depStatus.insufficientEvidence {
			records = append(records, findings.RuleExecutionRecord{
				RuleID:        ruleID,
				Applicability: findings.ApplicabilityApplicable,
				State:         findings.ExecutionInsufficientEvidence,
				Reason:        depStatus.reason,
			})
			continue
		}
		records = append(records, findings.RuleExecutionRecord{
			RuleID:        ruleID,
			Applicability: findings.ApplicabilityApplicable,
			State:         findings.ExecutionEvaluated,
		})
	}
	return all, records, errors.Join(errs...)
}

func evaluateRuleSafely(rule Rule, sc *ScanContext, targetVersion string) (fs []findings.Finding, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("rule panic: %v", r)
		}
	}()
	return rule.Evaluate(sc, targetVersion)
}

type dependencyStatus struct {
	notApplicable        bool
	missingPlane         bool
	insufficientEvidence bool
	reason               string
}

func evaluateRuleDependencies(rule Rule, sc *ScanContext) dependencyStatus {
	deps := ruleEvidenceDependencies(rule, sc)
	if len(deps) == 0 {
		return dependencyStatus{}
	}
	var unavailable []string
	var partial []string
	var applicable bool
	for _, dep := range deps {
		state := dependencyState(dep, sc)
		switch state {
		case dependencyAvailable:
			applicable = true
		case dependencyPartial:
			applicable = true
			partial = append(partial, dep.Label())
		case dependencyMissing:
			applicable = true
			unavailable = append(unavailable, dep.Label())
		case dependencyNotApplicable:
			// Keep looking. Some rules have alternative evidence planes
			// (for example add-on catalog checks can use live workloads even
			// when AWS was not requested).
		}
	}
	if !applicable {
		return dependencyStatus{notApplicable: true, reason: "required evidence plane was not requested for this scan mode"}
	}
	if len(unavailable) > 0 {
		return dependencyStatus{missingPlane: true, reason: "required evidence was unavailable for: " + strings.Join(sortedStrings(unavailable), ", ")}
	}
	if len(partial) > 0 {
		return dependencyStatus{insufficientEvidence: true, reason: "required collector data was unavailable for: " + strings.Join(sortedStrings(partial), ", ")}
	}
	return dependencyStatus{}
}

func ruleEvidenceDependencies(rule Rule, sc *ScanContext) []EvidenceDependency {
	if depRule, ok := rule.(ContextDependencyRule); ok {
		return depRule.EvidenceDependenciesFor(sc)
	}
	if depRule, ok := rule.(DependencyRule); ok {
		return depRule.EvidenceDependencies()
	}
	return nil
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

var (
	ruleErrorBearerTokenPattern  = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
	ruleErrorTokenParamPattern   = regexp.MustCompile(`(?i)(token|access_token|id_token|client_secret)=([^&\s]+)`)
	ruleErrorURLPattern          = regexp.MustCompile(`https?://[^\s)]+`)
	ruleErrorARNPattern          = regexp.MustCompile(`arn:aws[a-zA-Z-]*:[^\s)]+`)
	ruleErrorAWSAccountPattern   = regexp.MustCompile(`\b\d{12}\b`)
	ruleErrorPrivateIPPattern    = regexp.MustCompile(`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[0-1])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b`)
	ruleErrorEC2HostPattern      = regexp.MustCompile(`\bip-\d{1,3}(?:-\d{1,3}){3}[A-Za-z0-9.-]*\b`)
	ruleErrorInternalHostPattern = regexp.MustCompile(`\b[A-Za-z0-9.-]+\.internal\b`)
	ruleErrorUnixPathPattern     = regexp.MustCompile(`(^|[\s:])/(?:home|Users|var|etc|tmp|private|mnt|workspace|root)/[^\s)]+`)
	ruleErrorWindowsPathPattern  = regexp.MustCompile(`(?i)\b[A-Z]:\\[^\s)]+`)
)

// sanitizeRuleError renders err's text for a RuleExecutionRecord.Reason with
// lightweight secret/endpoint redaction. Rule errors are usually static
// operator-facing strings, but this keeps the execution-record path safe if
// a future rule wraps a lower-level client error.
func sanitizeRuleError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "rule returned an error with no message"
	}
	msg = ruleErrorBearerTokenPattern.ReplaceAllString(msg, "Bearer [REDACTED]")
	msg = ruleErrorTokenParamPattern.ReplaceAllString(msg, "$1=[REDACTED]")
	msg = ruleErrorURLPattern.ReplaceAllString(msg, "[REDACTED_URL]")
	msg = ruleErrorARNPattern.ReplaceAllString(msg, "[REDACTED_ARN]")
	msg = ruleErrorAWSAccountPattern.ReplaceAllString(msg, "[REDACTED_AWS_ACCOUNT]")
	msg = ruleErrorPrivateIPPattern.ReplaceAllString(msg, "[REDACTED_PRIVATE_IP]")
	msg = ruleErrorEC2HostPattern.ReplaceAllString(msg, "[REDACTED_PRIVATE_HOSTNAME]")
	msg = ruleErrorInternalHostPattern.ReplaceAllString(msg, "[REDACTED_PRIVATE_HOSTNAME]")
	msg = ruleErrorUnixPathPattern.ReplaceAllString(msg, "${1}[REDACTED_PATH]")
	msg = ruleErrorWindowsPathPattern.ReplaceAllString(msg, "[REDACTED_PATH]")
	return msg
}
