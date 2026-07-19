package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/imneeteeshyadav98/kubepreflight/internal/apicatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/compatcatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rollback"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
	"github.com/imneeteeshyadav98/kubepreflight/internal/v1compat"
)

// V1CompatibilityActual returns the current implementation surface checked
// against internal/v1compat's frozen v1 contract. It has no side effects and
// does not execute any command.
func V1CompatibilityActual() v1compat.Actual {
	ruleIDs := rules.NewDefaultRegistry().RuleIDs()
	priorities := make(map[string]string, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		priorities[ruleID] = findings.AssignPriority(findings.Finding{RuleID: ruleID}).Priority
	}

	return v1compat.Actual{
		Commands:          commandSurface(),
		SchemaVersions:    schemaVersions(),
		RuleIDs:           ruleIDs,
		DefaultPriorities: priorities,
		FingerprintV2Sample: findings.FingerprintV2(
			"API-001",
			"1.36",
			"removed-api",
			findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "uid-a"),
			findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "k8s/pdb.yaml"),
		),
		IncompleteResult:          incompleteReport().Result(),
		IncompleteExitCode:        incompleteReport().ExitCode(),
		BlockerResult:             blockerReport().Result(),
		BlockerExitCode:           blockerReport().ExitCode(),
		WarningResult:             warningReport().Result(),
		WarningExitCode:           warningReport().ExitCode(),
		CleanResult:               cleanReport().Result(),
		CleanExitCode:             cleanReport().ExitCode(),
		InfraFailureExitCode:      exitCodeForError(infraFailure(fmt.Errorf("cluster unavailable")), 0),
		GenericErrorExitCode:      exitCodeForError(fmt.Errorf("bad flags"), 0),
		CompareGateFailExitCode:   1,
		RollbackPreferredExit:     rollbackExitCode(rollbackAssessment(rollback.RecommendationRollbackPreferred)),
		RollbackDoNotProceedExit:  rollbackExitCode(rollbackAssessment(rollback.RecommendationDoNotProceed)),
		RollbackNeedsOperatorExit: rollbackExitCode(rollbackAssessment(rollback.RecommendationOperatorDecisionRequired)),
	}
}

func schemaVersions() map[string]string {
	return map[string]string{
		"findings":         findings.SchemaVersion,
		"plan":             findings.SchemaVersion,
		"actionPlan":       plan.ActionPlanSchemaVersion,
		"comparison":       comparison.SchemaVersion,
		"rollbackExcluded": rollback.SchemaVersion,
		"apiCatalog":       apicatalog.VersionedSchemaVersion,
		"compatCatalog":    compatcatalog.SchemaVersion,
	}
}

func commandSurface() []v1compat.Command {
	root := newRootCmd(new(int))
	var out []v1compat.Command
	visitCommand(root, nil, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func visitCommand(cmd *cobra.Command, parent []string, out *[]v1compat.Command) {
	pathParts := append(append([]string{}, parent...), cmd.Name())
	path := ""
	for _, part := range pathParts {
		if part != "" {
			if path != "" {
				path += " "
			}
			path += part
		}
	}
	aliases := append([]string{}, cmd.Aliases...)
	sort.Strings(aliases)
	*out = append(*out, v1compat.Command{Path: path, Aliases: aliases, Flags: commandFlags(path, cmd)})
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		visitCommand(child, pathParts, out)
	}
}

func commandFlags(path string, cmd *cobra.Command) []v1compat.Flag {
	var flags []v1compat.Flag
	cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name == "help" || flag.Name == "version" {
			return
		}
		flags = append(flags, v1compat.Flag{
			Name:     flag.Name,
			Default:  flag.DefValue,
			Required: requiredContractFlags()[path+"\x00"+flag.Name],
		})
	})
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func requiredContractFlags() map[string]bool {
	required := map[string]bool{}
	for _, key := range []string{
		"kubepreflight scan\x00target-version",
		"kubepreflight plan\x00to-version",
		"kubepreflight compare\x00baseline",
		"kubepreflight compare\x00current",
		"kubepreflight rollback plan\x00cluster-name",
		"kubepreflight rollback plan\x00provider",
		"kubepreflight rollback assess\x00cluster-name",
		"kubepreflight rollback assess\x00provider",
	} {
		required[key] = true
	}
	return required
}

func cleanReport() *findings.Report {
	return findings.NewReport("1.36", "", "", time.Time{}, nil)
}

func warningReport() *findings.Report {
	return findings.NewReport("1.36", "", "", time.Time{}, []findings.Finding{
		contractFinding("API-002", findings.SeverityWarning, "warn"),
	})
}

func blockerReport() *findings.Report {
	return findings.NewReport("1.36", "", "", time.Time{}, []findings.Finding{
		contractFinding("API-001", findings.SeverityBlocker, "block"),
	})
}

func incompleteReport() *findings.Report {
	r := cleanReport()
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoveragePartial, Errors: []string{"kubernetes: timeout"}},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	return r
}

func contractFinding(ruleID string, severity findings.Severity, discriminator string) findings.Finding {
	ref := findings.LiveResource("ConfigMap", findings.ScopeNamespaced, "default", discriminator, "uid-"+discriminator)
	return findings.Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  findings.TierStaticCertain,
		Message:     discriminator,
		Resources:   []findings.ResourceReference{ref},
		Fingerprint: findings.FingerprintV2(ruleID, "1.36", discriminator, ref),
	}
}

func rollbackAssessment(decision rollback.RecommendationDecision) rollback.Assessment {
	return rollback.Assessment{
		Recommendation: rollback.Recommendation{Decision: decision},
	}
}
