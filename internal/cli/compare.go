package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kubepreflight/internal/comparison"
)

// newCompareCmd wires `kubepreflight compare`, following the same
// error/exit-code conventions as scan/plan: an ordinary error (bad flags,
// a malformed input document) exits 1; infraFailure (a filesystem/runtime
// problem, not a document problem) exits 4. Unlike scan/plan, compare
// never has a report-derived exit code of its own -- there's no
// equivalent of "the scan found warnings" here, so it doesn't need the
// *exitCode out-parameter those commands use.
func newCompareCmd() *cobra.Command {
	var baselinePath string
	var currentPath string
	var jsonOut string
	var markdownOut string

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

			return nil
		},
	}

	cmd.Flags().StringVar(&baselinePath, "baseline", "", "path to the earlier scan's findings.json (required)")
	cmd.Flags().StringVar(&currentPath, "current", "", "path to the later scan's findings.json (required)")
	cmd.Flags().StringVar(&jsonOut, "json-out", "", "path to write the comparison as JSON (at least one of --json-out/--markdown-out is required)")
	cmd.Flags().StringVar(&markdownOut, "markdown-out", "", "path to write the comparison as a Markdown checklist (at least one of --json-out/--markdown-out is required)")

	return cmd
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
