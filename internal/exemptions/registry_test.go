package exemptions

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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
	root := filepath.Join("..", "..")
	for _, entry := range Registry() {
		path, anchor, ok := strings.Cut(entry.Documentation, "#")
		if !ok {
			t.Errorf("%s: documentation %q has no anchor", entry.ID, entry.Documentation)
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Errorf("%s: reading documentation %q: %v", entry.ID, entry.Documentation, err)
			continue
		}
		needle := "id=\"" + anchor + "\""
		if !strings.Contains(string(b), needle) {
			t.Errorf("%s: documentation %q missing anchor marker %q", entry.ID, entry.Documentation, needle)
		}
	}
}

func TestRegistryReferencedTestsExist(t *testing.T) {
	testNames, err := collectGoTestNames(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range Registry() {
		for _, ref := range append(append(append(append([]string{}, entry.PositiveTests...), entry.NegativeTests...), entry.SpoofingTests...), entry.PlaneTests...) {
			testName, _, _ := strings.Cut(ref, "/")
			if !testNames[testName] {
				t.Errorf("%s: referenced test %q does not exist", entry.ID, ref)
			}
		}
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

func collectGoTestNames(root string) (map[string]bool, error) {
	names := map[string]bool{}
	testFunc := regexp.MustCompile(`func\s+(Test[[:alnum:]_]+)\s*\(`)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "web", "demo":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range testFunc.FindAllSubmatch(b, -1) {
			names[string(match[1])] = true
		}
		return nil
	})
	return names, err
}
