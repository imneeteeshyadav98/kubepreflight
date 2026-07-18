package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kubepreflight/internal/comparison"
	"kubepreflight/internal/gate"
	"kubepreflight/internal/redact"
)

// validWarningPolicies are the only accepted --warning-policy values,
// matching gate.WarningPolicy's three constants exactly.
var validWarningPolicies = map[string]bool{
	string(gate.WarningPolicyIgnore):    true,
	string(gate.WarningPolicyFailOnNew): true,
	string(gate.WarningPolicyFailOnAny): true,
}

// newCompareCmd wires `kubepreflight compare`, following the same
// error/exit-code conventions as scan/plan: an ordinary error (bad flags,
// a malformed input document) exits 1; infraFailure (a filesystem/runtime
// problem, not a document problem) exits 4. Unlike scan/plan, compare has
// no report-derived exit code UNLESS --gate-out is set -- with no gate
// requested, the command always succeeds (exit 0) once its inputs are
// valid, exactly as before this flag existed. Only when a caller opts
// into gate evaluation does *exitCode start reflecting the decision (0
// for pass/neutral, 1 for fail), the same out-parameter pattern
// scan/plan/rollback already use for their own report-derived codes.
func newCompareCmd(exitCode *int) *cobra.Command {
	var baselinePath string
	var currentPath string
	var jsonOut string
	var markdownOut string
	var gateOut string
	var failOnNewBlockers bool
	var warningPolicy string
	var failOnVerdictRegression bool
	var minimumScoreDelta int
	var redactSensitiveIdentifiers bool

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two findings.json scans and show upgrade-readiness progress",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if baselinePath == "" {
				return fmt.Errorf("--baseline is required")
			}
			if currentPath == "" {
				return fmt.Errorf("--current is required")
			}
			if jsonOut == "" && markdownOut == "" {
				return fmt.Errorf("at least one of --json-out or --markdown-out is required")
			}
			if !validWarningPolicies[warningPolicy] {
				return fmt.Errorf("--warning-policy %q is not supported (use ignore, fail_on_new, or fail_on_any)", warningPolicy)
			}

			baselineRaw, err := os.ReadFile(baselinePath)
			if err != nil {
				return infraFailure(fmt.Errorf("reading --baseline: %w", err))
			}
			currentRaw, err := os.ReadFile(currentPath)
			if err != nil {
				return infraFailure(fmt.Errorf("reading --current: %w", err))
			}

			baseline, err := comparison.LoadAndNormalize(baselineRaw)
			if err != nil {
				return fmt.Errorf("--baseline: %w", err)
			}
			current, err := comparison.LoadAndNormalize(currentRaw)
			if err != nil {
				return fmt.Errorf("--current: %w", err)
			}

			cmp, err := comparison.Compare(baseline, current)
			if err != nil {
				return err
			}
			// Redacts cmp only, not baseline/current -- gate.Evaluate below
			// reads the real (unredacted) baseline/current, but gate.Result
			// carries only counts and typed reason codes, never finding
			// text, so that's never a leak path. This also covers the case
			// where --baseline/--current are themselves unredacted
			// findings.json files: the operator's intent to share, signaled
			// by passing this flag on compare specifically, still holds
			// even though the flag wasn't passed to the original scan.
			if redactSensitiveIdentifiers {
				redact.Comparison(cmp)
			}

			if jsonOut != "" {
				if err := writeComparisonJSONFile(jsonOut, cmp); err != nil {
					return infraFailure(err)
				}
			}
			if markdownOut != "" {
				if err := writeComparisonMarkdownFile(markdownOut, cmp); err != nil {
					return infraFailure(err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Comparison: %s -> %s", cmp.Summary.BaselineVerdict, cmp.Summary.CurrentVerdict)
			if cmp.Summary.VerdictChanged {
				fmt.Fprint(cmd.OutOrStdout(), " (changed)")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nReadiness score: %d -> %d\n", cmp.Summary.BaselineReadinessScore, cmp.Summary.CurrentReadinessScore)
			fmt.Fprintf(cmd.OutOrStdout(), "New: %d (%d blocker(s))  Resolved: %d (%d blocker(s))  Changed: %d  Unchanged: %d\n",
				cmp.Summary.New, cmp.Summary.NewBlockers, cmp.Summary.Resolved, cmp.Summary.ResolvedBlockers, cmp.Summary.Changed, cmp.Summary.Unchanged)
			for _, warning := range cmp.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning)
			}

			if gateOut != "" {
				policy := gate.Policy{
					FailOnNewBlockers:       failOnNewBlockers,
					WarningPolicy:           gate.WarningPolicy(warningPolicy),
					FailOnVerdictRegression: failOnVerdictRegression,
					MinimumScoreDelta:       minimumScoreDelta,
				}
				result := gate.Evaluate(baseline, current, cmp, policy)
				if err := writeGateJSONFile(gateOut, result); err != nil {
					return infraFailure(err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Gate decision: %s", result.Decision)
				if len(result.Reasons) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), " (%v)", result.Reasons)
				}
				fmt.Fprintln(cmd.OutOrStdout())
				// neutral never blocks CI -- insufficient evidence is a
				// reason to look closer, not a reason to fail a merge for
				// something the gate couldn't actually confirm regressed.
				if result.Decision == gate.DecisionFail {
					*exitCode = 1
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&baselinePath, "baseline", "", "path to the earlier scan's findings.json (required)")
	cmd.Flags().StringVar(&currentPath, "current", "", "path to the later scan's findings.json (required)")
	cmd.Flags().StringVar(&jsonOut, "json-out", "", "path to write the comparison as JSON (at least one of --json-out/--markdown-out is required)")
	cmd.Flags().StringVar(&markdownOut, "markdown-out", "", "path to write the comparison as a Markdown checklist (at least one of --json-out/--markdown-out is required)")
	cmd.Flags().StringVar(&gateOut, "gate-out", "", "path to write a gate decision (pass/fail/neutral) as JSON; omit to skip gate evaluation entirely")
	cmd.Flags().BoolVar(&failOnNewBlockers, "fail-on-new-blockers", true, "fail the gate when the current scan introduces a new Blocker-severity finding")
	cmd.Flags().StringVar(&warningPolicy, "warning-policy", string(gate.WarningPolicyIgnore), "how warnings affect the gate: ignore, fail_on_new, or fail_on_any")
	cmd.Flags().BoolVar(&failOnVerdictRegression, "fail-on-verdict-regression", true, "fail the gate when the overall verdict gets strictly worse (e.g. CLEAN -> BLOCKED)")
	cmd.Flags().IntVar(&minimumScoreDelta, "minimum-score-delta", 0, "lowest readiness-score movement (current minus baseline) that still passes the gate")
	cmd.Flags().BoolVar(&redactSensitiveIdentifiers, "redact-sensitive-identifiers", false, "replace AWS ARNs and EC2-style internal node hostnames with placeholders in the comparison output (JSON/Markdown/terminal) — use before sharing generated evidence outside your organization, even if --baseline/--current were not themselves redacted; does not change comparison results, gate decisions, or exit codes")

	return cmd
}

func writeGateJSONFile(path string, result gate.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func writeComparisonJSONFile(path string, cmp *comparison.Comparison) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cmp); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func writeComparisonMarkdownFile(path string, cmp *comparison.Comparison) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	if err := comparison.WriteMarkdown(cmp, f); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
