package v1compat

import (
	"strings"
	"testing"
)

func TestCheckDetectsMissingCommand(t *testing.T) {
	actual := baselineActual()
	actual.Commands = actual.Commands[:len(actual.Commands)-1]
	report := Check(actual)
	if !hasIssue(report, "missing command") {
		t.Fatalf("Check() issues = %v, want missing command issue", report.Issues)
	}
}

func TestCheckDetectsFlagDefaultDrift(t *testing.T) {
	actual := baselineActual()
	for i := range actual.Commands {
		if actual.Commands[i].Path == "kubepreflight scan" {
			for j := range actual.Commands[i].Flags {
				if actual.Commands[i].Flags[j].Name == "output" {
					actual.Commands[i].Flags[j].Default = "all"
				}
			}
		}
	}
	report := Check(actual)
	if !hasIssue(report, "kubepreflight scan --output default") {
		t.Fatalf("Check() issues = %v, want flag default issue", report.Issues)
	}
}

func TestCheckDetectsSchemaDrift(t *testing.T) {
	actual := baselineActual()
	actual.SchemaVersions["comparison"] = "kubepreflight.io/scan-comparison/v2"
	report := Check(actual)
	if !hasIssue(report, "comparison schemaVersion") {
		t.Fatalf("Check() issues = %v, want schema issue", report.Issues)
	}
}

func TestCheckDetectsRuleIDDrift(t *testing.T) {
	actual := baselineActual()
	actual.RuleIDs = append(actual.RuleIDs, "NEW-001")
	actual.DefaultPriorities["NEW-001"] = "P4"
	report := Check(actual)
	if !hasIssue(report, "registered rule IDs") {
		t.Fatalf("Check() issues = %v, want rule ID issue", report.Issues)
	}
}

func TestCheckDetectsFingerprintDrift(t *testing.T) {
	actual := baselineActual()
	actual.FingerprintV2Sample = "changed"
	report := Check(actual)
	if !hasIssue(report, "FingerprintV2 sample") {
		t.Fatalf("Check() issues = %v, want fingerprint issue", report.Issues)
	}
}

func baselineActual() Actual {
	return Actual{
		Commands:                  ExpectedCommands(),
		SchemaVersions:            baselineSchemas(),
		RuleIDs:                   ExpectedRuleIDs(),
		DefaultPriorities:         ExpectedDefaultPriorities(),
		FingerprintV2Sample:       "82cbaec03e4fd838b1ce5b9eda1c4d297f0bc05db73c0632b379813912bb8a40",
		IncompleteResult:          "INCOMPLETE",
		IncompleteExitCode:        3,
		BlockerResult:             "BLOCKED",
		BlockerExitCode:           2,
		WarningResult:             "PASSED_WITH_WARNINGS",
		WarningExitCode:           1,
		CleanResult:               "CLEAN",
		CleanExitCode:             0,
		InfraFailureExitCode:      4,
		GenericErrorExitCode:      1,
		CompareGateFailExitCode:   1,
		RollbackPreferredExit:     0,
		RollbackDoNotProceedExit:  2,
		RollbackNeedsOperatorExit: 1,
	}
}

func baselineSchemas() map[string]string {
	return map[string]string{
		"findings":         StableScanSchemaVersion,
		"plan":             StableScanSchemaVersion,
		"actionPlan":       StableActionPlanSchemaVersion,
		"comparison":       StableComparisonSchemaVersion,
		"rollbackExcluded": RollbackSchemaVersion,
		"apiCatalog":       "apicatalog.kubepreflight.io/v1",
		"compatCatalog":    "compatcatalog.kubepreflight.io/v1",
	}
}

func hasIssue(report Report, text string) bool {
	for _, issue := range report.Issues {
		if strings.Contains(issue.Message, text) {
			return true
		}
	}
	return false
}
