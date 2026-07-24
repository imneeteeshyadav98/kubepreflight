package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/redact"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
	rollbackeks "github.com/imneeteeshyadav98/kubepreflight/internal/rollback/eks"
)

func newRollbackCmd(exitCode *int) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Assess EKS rollback readiness without executing rollback",
		Long:  "Assess whether an EKS control-plane rollback is eligible, operationally ready, and preferable to fix-forward. The command is read-only and never executes rollback or mutates cluster resources.",
	}
	cmd.AddCommand(newRollbackAssessmentCmd("plan", rollback.ModePreUpgradePosture, exitCode))
	cmd.AddCommand(newRollbackAssessmentCmd("assess", rollback.ModePostUpgradeReadiness, exitCode))
	return cmd
}

func newRollbackAssessmentCmd(name string, mode rollback.AssessmentMode, exitCode *int) *cobra.Command {
	var provider string
	var clusterName string
	var output string
	var outputDir string
	var assessmentOut string
	var findingsPath string
	var terminalOutput string
	var collectorTimeout time.Duration
	var redactSensitiveIdentifiers bool

	cmd := &cobra.Command{
		Use:   name,
		Short: rollbackCommandShort(mode),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider != "eks" {
				return fmt.Errorf("--provider %q is not supported for rollback readiness (use eks)", provider)
			}
			if clusterName == "" {
				return fmt.Errorf("--cluster-name is required")
			}
			if !validOutputs[output] {
				return fmt.Errorf("--output %q is not supported (use json, md, html, or all)", output)
			}
			if !validTerminalOutputs[terminalOutput] {
				return fmt.Errorf("--terminal-output %q is not supported (use compact, full, or silent)", terminalOutput)
			}

			// Validate --findings before any EKS collection is attempted: a
			// malformed or wrong-document findings input is a CLI
			// input/infrastructure failure, and failing on it here means it
			// is caught before spending an AWS round trip, not merely
			// before rollback.ApplyOperationalReadiness runs later.
			var findingsReport *findings.Report
			if findingsPath != "" {
				rpt, err := readFindingsReport(findingsPath)
				if err != nil {
					return infraFailure(fmt.Errorf("reading --findings %s: %w", findingsPath, err))
				}
				findingsReport = rpt
			}

			collectCtx, stopCollectSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopCollectSignals()

			collector, err := rollbackeks.LoadCollector(collectCtx, clusterName)
			if err != nil {
				return infraFailure(fmt.Errorf("loading EKS rollback collector: %w", err))
			}
			snap, err := collector.Collect(collectCtx, collectorTimeout, time.Now().UTC())
			if err != nil {
				return infraFailure(fmt.Errorf("collecting EKS rollback evidence: %w", err))
			}

			assessment := rollbackeks.EvaluateEligibility(snap, time.Now().UTC())
			assessment.Mode = mode
			assessment = rollbackeks.ApplyRollbackInsights(assessment, snap, time.Now().UTC())

			assessment = rollback.ApplyOperationalReadiness(assessment, findingsReport)
			assessment = rollback.ApplyRecommendation(assessment)
			if err := assessment.Validate(); err != nil {
				return fmt.Errorf("building rollback assessment: %w", err)
			}
			if redactSensitiveIdentifiers {
				redact.RollbackAssessment(&assessment)
			}

			*exitCode = rollbackExitCode(assessment)

			if terminalOutput != "silent" {
				if err := report.WriteRollbackTerminal(&assessment, cmd.OutOrStdout()); err != nil {
					return fmt.Errorf("rendering rollback summary: %w", err)
				}
			}

			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			var written []string
			for _, target := range rollbackReportTargets(output, outputDir, assessmentOut) {
				if err := writeRollbackReportFile(target.path, &assessment, target.write); err != nil {
					return err
				}
				written = append(written, target.path)
			}
			if terminalOutput != "silent" {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRollback reports written:")
				for _, path := range written {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", path)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "eks", "cloud provider for rollback readiness assessment (only eks is supported)")
	cmd.Flags().StringVar(&clusterName, "cluster-name", "", "EKS cluster name")
	cmd.Flags().StringVar(&output, "output", "json", "output format: json, md, html, or all")
	cmd.Flags().StringVar(&outputDir, "output-dir", ".", "directory for generated rollback report artifacts")
	cmd.Flags().StringVar(&assessmentOut, "assessment-out", "rollback-assessment.json", "path for rollback assessment JSON")
	cmd.Flags().StringVar(&findingsPath, "findings", "", "optional findings.json from a recent scan to include operational readiness evidence")
	cmd.Flags().StringVar(&terminalOutput, "terminal-output", "full", "stdout detail level: compact, full, or silent")
	cmd.Flags().DurationVar(&collectorTimeout, "collector-timeout", k8s.DefaultCollectorTimeout, "per-call timeout for EKS rollback evidence collection")
	cmd.Flags().BoolVar(&redactSensitiveIdentifiers, "redact-sensitive-identifiers", false, "replace AWS ARNs and EC2-style internal node hostnames with placeholders in every output (rollback-assessment.json, rollback-report.md/html, terminal) — use before sharing generated evidence outside your organization; does not change the assessment, recommendation, or exit code")
	return cmd
}

func rollbackCommandShort(mode rollback.AssessmentMode) string {
	if mode == rollback.ModePreUpgradePosture {
		return "Assess whether an operational rollback path is likely to remain open"
	}
	return "Assess current EKS rollback readiness after an upgrade"
}

type rollbackReportTarget struct {
	path  string
	write func(*rollback.Assessment, io.Writer) error
}

func rollbackReportTargets(output, outputDir, assessmentOut string) []rollbackReportTarget {
	assessmentPath := resolveOutputPath(outputDir, assessmentOut)
	targets := []rollbackReportTarget{{path: assessmentPath, write: report.WriteRollbackJSON}}
	switch output {
	case "md":
		targets = append(targets, rollbackReportTarget{path: filepath.Join(outputDir, "rollback-report.md"), write: report.WriteRollbackMarkdown})
	case "html":
		targets = append(targets, rollbackReportTarget{path: filepath.Join(outputDir, "rollback-report.html"), write: report.WriteRollbackHTML})
	case "all":
		targets = append(targets,
			rollbackReportTarget{path: filepath.Join(outputDir, "rollback-report.md"), write: report.WriteRollbackMarkdown},
			rollbackReportTarget{path: filepath.Join(outputDir, "rollback-report.html"), write: report.WriteRollbackHTML},
		)
	}
	return targets
}

func writeRollbackReportFile(path string, assessment *rollback.Assessment, write func(*rollback.Assessment, io.Writer) error) error {
	f, err := createReportFile(path)
	if err != nil {
		return err
	}
	if err := write(assessment, f); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", path, err)
	}
	return nil
}

// readFindingsReport loads and validates a KubePreflight findings.json
// document supplied via --findings. A wrong or malformed document here is a
// CLI input/infrastructure failure, not a valid-but-empty scan or an
// insufficient-evidence report: Go's JSON decoder silently ignores unknown
// fields and leaves absent fields at their zero value, so without this
// validation an unrelated document (a rollback Assessment, a comparison
// Comparison, a Kubernetes object, `{}`, `null`, ...) would decode
// "successfully" into a mostly-empty findings.Report and reach
// rollback.ApplyOperationalReadiness, producing a reassuring verdict from
// evidence that was never actually a findings report. See
// validateRollbackFindingsDocument for the specific invariants enforced.
func readFindingsReport(path string) (*findings.Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var rpt findings.Report
	if err := dec.Decode(&rpt); err != nil {
		return nil, fmt.Errorf("invalid --findings document: %w", err)
	}

	// Require exactly one JSON document. A second successful decode means
	// another JSON value follows (e.g. two concatenated objects); a
	// non-EOF error means trailing malformed content follows. Ordinary
	// trailing whitespace/newlines after the single document are the only
	// case where the second Decode call correctly returns io.EOF.
	var trailing json.RawMessage
	switch err := dec.Decode(&trailing); {
	case errors.Is(err, io.EOF):
		// exactly one document -- expected case, fall through.
	case err == nil:
		return nil, fmt.Errorf("invalid --findings document: trailing JSON content is not allowed")
	default:
		return nil, fmt.Errorf("invalid --findings document: trailing JSON content is not allowed: %w", err)
	}

	if err := validateRollbackFindingsDocument(rpt); err != nil {
		return nil, err
	}

	return &rpt, nil
}

// validateRollbackFindingsDocument enforces the minimal structural
// invariants that distinguish a genuine KubePreflight findings report from
// any other JSON document, without imposing requirements beyond that. It
// intentionally validates only three things -- schemaVersion, targetVersion,
// and the presence (not contents) of the findings collection -- because
// every other findings.Report field is legitimately absent on some genuine
// report variant (manifest-only scans have no live cluster identity,
// partial-collection scans lack provider enrichment, clean scans have zero
// findings, and so on). Provenance/freshness gates beyond this input
// boundary are handled separately by validateAPIEvidenceTarget,
// validateClusterEvidenceIdentity, and validateFindingsFreshness in
// internal/rollback/operational.go and are unaffected by this check.
func validateRollbackFindingsDocument(report findings.Report) error {
	if strings.TrimSpace(report.SchemaVersion) != findings.SchemaVersion {
		return fmt.Errorf("invalid --findings document: expected KubePreflight findings schema %s, got %q", findings.SchemaVersion, report.SchemaVersion)
	}
	if strings.TrimSpace(report.TargetVersion) == "" {
		return errors.New("invalid --findings document: targetVersion is required")
	}
	if report.Findings == nil {
		return errors.New("invalid --findings document: findings must be a JSON array")
	}
	return nil
}

func rollbackExitCode(assessment rollback.Assessment) int {
	switch assessment.Recommendation.Decision {
	case rollback.RecommendationRollbackPreferred:
		return 0
	case rollback.RecommendationDoNotProceed:
		return 2
	default:
		return 1
	}
}
