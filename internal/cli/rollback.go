package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/redact"
	"kubepreflight/internal/report"
	"kubepreflight/internal/rollback"
	rollbackeks "kubepreflight/internal/rollback/eks"
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

			if findingsPath != "" {
				rpt, err := readFindingsReport(findingsPath)
				if err != nil {
					return fmt.Errorf("reading --findings %s: %w", findingsPath, err)
				}
				assessment = rollback.ApplyOperationalReadiness(assessment, rpt)
			} else {
				assessment = rollback.ApplyOperationalReadiness(assessment, nil)
			}
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
	assessmentPath := assessmentOut
	if !filepath.IsAbs(assessmentPath) {
		assessmentPath = filepath.Join(outputDir, assessmentPath)
	}
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

func readFindingsReport(path string) (*findings.Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rpt findings.Report
	if err := json.NewDecoder(f).Decode(&rpt); err != nil {
		return nil, err
	}
	return &rpt, nil
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
