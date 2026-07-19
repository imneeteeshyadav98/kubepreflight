package exemptions

import "testing"

func TestRegistryGovernanceComplete(t *testing.T) {
	if errs := Validate(Registry()); len(errs) > 0 {
		for _, err := range errs {
			t.Error(err)
		}
	}
}

func TestRegistryOrderingIsDeterministic(t *testing.T) {
	entries := Registry()
	for i := 1; i < len(entries); i++ {
		if entries[i-1].ID > entries[i].ID {
			t.Fatalf("Registry() order is not deterministic by ID: %q before %q", entries[i-1].ID, entries[i].ID)
		}
	}
}

func TestRegistryReturnsDefensiveCopies(t *testing.T) {
	entries := Registry()
	if len(entries) == 0 {
		t.Fatal("Registry() returned no entries")
	}
	entries[0].ID = "mutated"
	entries[0].RequiredEvidence[0] = "mutated evidence"

	next := Registry()
	if next[0].ID == "mutated" || next[0].RequiredEvidence[0] == "mutated evidence" {
		t.Fatalf("Registry() returned shared mutable state: %+v", next[0])
	}

	got := MustGet(API001LiveEventsID)
	got.RequiredEvidence[0] = "mutated via lookup"
	if MustGet(API001LiveEventsID).RequiredEvidence[0] == "mutated via lookup" {
		t.Fatal("MustGet returned shared mutable state")
	}
}

func TestRegistryDocumentationAnchorsExist(t *testing.T) {
	for _, issue := range checkDocs("../..", Registry()) {
		t.Error(issue)
	}
}

func TestRegistryReferencedTestsExist(t *testing.T) {
	for _, issue := range checkReferencedTests("../..", Registry()) {
		t.Error(issue)
	}
}

func TestAuditInventoryGovernedEntriesResolveRegistryIDs(t *testing.T) {
	registryIDs := map[string]bool{}
	for _, entry := range Registry() {
		registryIDs[entry.ID] = true
	}

	hasGoverned := false
	hasNonExemption := false
	for _, entry := range AuditInventory() {
		if entry.Path == "" || entry.Function == "" || entry.Rationale == "" || entry.MigrationDecision == "" {
			t.Errorf("incomplete audit entry: %+v", entry)
		}
		if len(entry.AffectedRules) == 0 || len(entry.RequiredEvidence) == 0 {
			t.Errorf("audit entry missing rules/evidence: %+v", entry)
		}
		if !validPlane(entry.EvaluationPlane) {
			t.Errorf("audit entry has invalid plane: %+v", entry)
		}
		switch entry.Classification {
		case AuditGovernedExemption:
			hasGoverned = true
			if !registryIDs[entry.RegistryID] {
				t.Errorf("governed audit entry references unknown registry ID %q: %+v", entry.RegistryID, entry)
			}
		case AuditNonExemption, AuditSeparatePath:
			hasNonExemption = true
			if entry.RegistryID != "" {
				t.Errorf("non-governed audit entry must not reference a registry ID: %+v", entry)
			}
		default:
			t.Errorf("unknown audit classification %q: %+v", entry.Classification, entry)
		}
	}
	if !hasGoverned || !hasNonExemption {
		t.Fatalf("audit inventory must include governed and non-exemption paths; governed=%t nonExemption=%t", hasGoverned, hasNonExemption)
	}
}
