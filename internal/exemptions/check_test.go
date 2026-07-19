package exemptions

import (
	"strings"
	"testing"
)

func TestCheckPassesRepositoryState(t *testing.T) {
	report := Check("../..")
	if !report.OK() {
		t.Fatalf("Check() failed:\n%s", report.String())
	}
}

func TestCheckDetectsDuplicateID(t *testing.T) {
	entries := Registry()
	entries = append(entries, entries[0])
	errs := Validate(entries)
	if !hasIssue(errs, "duplicate ID") {
		t.Fatalf("Validate duplicate ID errors = %v, want duplicate ID issue", errs)
	}
}

func TestCheckDetectsMissingRationaleEvidenceAndDocumentation(t *testing.T) {
	entry := MustGet(API001LiveEventsID)
	entry.Rationale = ""
	entry.RequiredEvidence = nil
	entry.Documentation = ""
	errs := Validate([]Entry{entry})
	for _, want := range []string{"missing Rationale", "missing RequiredEvidence", "missing Documentation"} {
		if !hasIssue(errs, want) {
			t.Fatalf("Validate errors = %v, want %q", errs, want)
		}
	}
}

func TestCheckDetectsStaleAuditEntry(t *testing.T) {
	entries := []Entry{MustGet(API001LiveEventsID)}
	var audit []AuditEntry
	issues := checkAuditInventory(entries, audit)
	if !hasCheckIssue(issues, "has no governed audit inventory row") {
		t.Fatalf("checkAuditInventory issues = %v, want stale registry issue", issues)
	}
}

func TestCheckDetectsUnknownProductionID(t *testing.T) {
	issues := checkAuditInventory(nil, []AuditEntry{{
		Path:              "internal/rules/api001.go",
		Function:          "isEphemeralEvent",
		AffectedRules:     []string{"API-001"},
		Classification:    AuditGovernedExemption,
		RegistryID:        "unknown-id",
		EvaluationPlane:   PlaneLive,
		RequiredEvidence:  []string{"evidence"},
		MigrationDecision: "test fixture",
		Rationale:         "test fixture",
	}})
	if !hasCheckIssue(issues, "unknown registry ID") {
		t.Fatalf("checkAuditInventory issues = %v, want unknown registry ID issue", issues)
	}
}

func TestCheckDetectsUnregisteredSuppressionCallsiteFixture(t *testing.T) {
	entry := AuditEntry{
		Path:              "internal/rules/api001.go",
		Function:          "isEphemeralEvent",
		AffectedRules:     []string{"API-001"},
		Classification:    AuditNonExemption,
		RegistryID:        API001LiveEventsID,
		EvaluationPlane:   PlaneLive,
		RequiredEvidence:  []string{"evidence"},
		MigrationDecision: "test fixture",
		Rationale:         "test fixture",
	}
	issues := checkAuditInventory(Registry(), []AuditEntry{entry})
	if !hasCheckIssue(issues, "non-governed audit entry references registry ID") {
		t.Fatalf("checkAuditInventory issues = %v, want unregistered callsite issue", issues)
	}
}

func hasIssue(errs []error, want string) bool {
	for _, err := range errs {
		if strings.Contains(err.Error(), want) {
			return true
		}
	}
	return false
}

func hasCheckIssue(issues []CheckIssue, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Message, want) {
			return true
		}
	}
	return false
}
