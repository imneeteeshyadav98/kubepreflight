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

// TestCheckPassesFindingsSchemaVersion1_1 confirms Check() is satisfied when
// the "findings"/"plan" schema identifiers are exactly "1.1" -- the v1.3.0
// PR 7 bump -- and fails loudly for any other value, including the old
// "1.0" (proving this is a real structural assertion against the current
// version, not a stale check that would still pass for the pre-bump value).
func TestCheckPassesFindingsSchemaVersion1_1(t *testing.T) {
	if StableScanSchemaVersion != "1.1" {
		t.Fatalf("StableScanSchemaVersion = %q, want %q", StableScanSchemaVersion, "1.1")
	}
	actual := baselineActual()
	report := Check(actual)
	if !report.OK() {
		t.Fatalf("Check() with schema 1.1 = %v, want OK", report.Issues)
	}

	regressed := baselineActual()
	regressed.SchemaVersions["findings"] = "1.0"
	report = Check(regressed)
	if !hasIssue(report, "findings schemaVersion") {
		t.Fatalf("Check() issues = %v, want a findings schemaVersion issue for a regressed 1.0 value", report.Issues)
	}
}

func TestCheckDetectsRuleApplicabilityDrift(t *testing.T) {
	actual := baselineActual()
	actual.RuleApplicabilityValues = []string{"applicable", "unknown"}
	report := Check(actual)
	if !hasIssue(report, "RuleApplicability wire values") {
		t.Fatalf("Check() issues = %v, want RuleApplicability wire value issue", report.Issues)
	}
}

func TestCheckDetectsRuleExecutionStateDrift(t *testing.T) {
	actual := baselineActual()
	actual.RuleExecutionStateValues = []string{"evaluated", "not_evaluated", "insufficient_evidence", "errored"}
	report := Check(actual)
	if !hasIssue(report, "RuleExecutionState wire values") {
		t.Fatalf("Check() issues = %v, want RuleExecutionState wire value issue", report.Issues)
	}
}

func TestCheckDetectsRuleExecutionRecordFieldDrift(t *testing.T) {
	actual := baselineActual()
	actual.RuleExecutionRecordFields = []string{"ruleId", "applicability", "state"}
	report := Check(actual)
	if !hasIssue(report, "RuleExecutionRecord JSON fields") {
		t.Fatalf("Check() issues = %v, want RuleExecutionRecord field issue", report.Issues)
	}
}

func TestCheckDetectsReportJSONFieldDrift(t *testing.T) {
	actual := baselineActual()
	actual.ReportJSONFields = actual.ReportJSONFields[:len(actual.ReportJSONFields)-1]
	report := Check(actual)
	if !hasIssue(report, "Report JSON fields") {
		t.Fatalf("Check() issues = %v, want Report JSON field issue", report.Issues)
	}
}

func TestCheckDetectsLegacyDocumentRejection(t *testing.T) {
	actual := baselineActual()
	actual.LegacyDocumentLoads = false
	report := Check(actual)
	if !hasIssue(report, "failed to load via comparison.LoadAndNormalize") {
		t.Fatalf("Check() issues = %v, want legacy document load issue", report.Issues)
	}
}

func TestCheckDetectsLegacyEvaluatedRegression(t *testing.T) {
	actual := baselineActual()
	actual.LegacyZeroFindingsNeverEvaluated = false
	report := Check(actual)
	if !hasIssue(report, "must never be read as a clean evaluation") {
		t.Fatalf("Check() issues = %v, want legacy conservative-normalization issue", report.Issues)
	}
}

func baselineActual() Actual {
	return Actual{
		Commands:                         ExpectedCommands(),
		SchemaVersions:                   baselineSchemas(),
		RuleIDs:                          ExpectedRuleIDs(),
		DefaultPriorities:                ExpectedDefaultPriorities(),
		FingerprintV2Sample:              "82cbaec03e4fd838b1ce5b9eda1c4d297f0bc05db73c0632b379813912bb8a40",
		IncompleteResult:                 "INCOMPLETE",
		IncompleteExitCode:               3,
		BlockerResult:                    "BLOCKED",
		BlockerExitCode:                  2,
		WarningResult:                    "PASSED_WITH_WARNINGS",
		WarningExitCode:                  1,
		CleanResult:                      "CLEAN",
		CleanExitCode:                    0,
		InfraFailureExitCode:             4,
		GenericErrorExitCode:             1,
		CompareGateFailExitCode:          1,
		RollbackPreferredExit:            0,
		RollbackDoNotProceedExit:         2,
		RollbackNeedsOperatorExit:        1,
		RuleApplicabilityValues:          ExpectedRuleApplicabilityValues(),
		RuleExecutionStateValues:         ExpectedRuleExecutionStateValues(),
		RuleExecutionRecordFields:        ExpectedRuleExecutionRecordFields(),
		ReportJSONFields:                 ExpectedReportJSONFields(),
		LegacyDocumentLoads:              true,
		LegacyZeroFindingsNeverEvaluated: true,
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
