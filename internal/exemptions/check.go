package exemptions

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type CheckIssue struct {
	Message string
}

func (i CheckIssue) Error() string {
	return i.Message
}

type CheckReport struct {
	RegistryEntries int
	AuditEntries    int
	Issues          []CheckIssue
}

func (r CheckReport) OK() bool {
	return len(r.Issues) == 0
}

func Check(root string) CheckReport {
	entries := Registry()
	audit := AuditInventory()
	report := CheckReport{RegistryEntries: len(entries), AuditEntries: len(audit)}
	report.addValidationErrors(Validate(entries))
	report.addIssues(checkDocs(root, entries)...)
	report.addIssues(checkReferencedTests(root, entries)...)
	report.addIssues(checkAffectedRules(entries, audit)...)
	report.addIssues(checkAuditInventory(entries, audit)...)
	report.addIssues(checkProductionReferences(root, entries, audit)...)
	sort.Slice(report.Issues, func(i, j int) bool {
		return report.Issues[i].Message < report.Issues[j].Message
	})
	return report
}

func (r *CheckReport) addValidationErrors(errs []error) {
	for _, err := range errs {
		r.Issues = append(r.Issues, CheckIssue{Message: err.Error()})
	}
}

func (r *CheckReport) addIssues(issues ...CheckIssue) {
	r.Issues = append(r.Issues, issues...)
}

func (r CheckReport) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "False-positive exemption governance: %d registry entrie(s), %d audit entrie(s)\n", r.RegistryEntries, r.AuditEntries)
	if len(r.Issues) == 0 {
		sb.WriteString("Governance check: OK\n")
		return sb.String()
	}
	sb.WriteString("Governance check: FAILED\n")
	for _, issue := range r.Issues {
		fmt.Fprintf(&sb, "  - %s\n", issue.Message)
	}
	return sb.String()
}

func checkDocs(root string, entries []Entry) []CheckIssue {
	var issues []CheckIssue
	for _, entry := range entries {
		path, anchor, ok := strings.Cut(entry.Documentation, "#")
		if !ok {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: documentation %q has no anchor", entry.ID, entry.Documentation)})
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: documentation %q cannot be read: %v", entry.ID, entry.Documentation, err)})
			continue
		}
		if !strings.Contains(string(b), `id="`+anchor+`"`) {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: documentation %q missing anchor", entry.ID, entry.Documentation)})
		}
	}
	return issues
}

func checkReferencedTests(root string, entries []Entry) []CheckIssue {
	names, err := collectGoTestNames(root)
	if err != nil {
		return []CheckIssue{{Message: fmt.Sprintf("cannot collect Go test names: %v", err)}}
	}
	var issues []CheckIssue
	for _, entry := range entries {
		for _, ref := range entry.allTests() {
			testName, _, _ := strings.Cut(ref, "/")
			if !names[testName] {
				issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: referenced test %q does not exist", entry.ID, ref)})
			}
		}
	}
	return issues
}

func checkAffectedRules(entries []Entry, audit []AuditEntry) []CheckIssue {
	known := map[string]bool{
		"all namespaced findings": true,
		"provider enrichment":     true,
	}
	for _, entry := range audit {
		for _, rule := range entry.AffectedRules {
			known[rule] = true
		}
	}
	var issues []CheckIssue
	for _, entry := range entries {
		for _, rule := range entry.AffectedRules {
			if !known[rule] {
				issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: affected rule %q is not known in audit inventory", entry.ID, rule)})
			}
		}
	}
	return issues
}

func checkAuditInventory(entries []Entry, audit []AuditEntry) []CheckIssue {
	registryIDs := map[string]bool{}
	governedIDs := map[string]bool{}
	var issues []CheckIssue
	for _, entry := range entries {
		registryIDs[entry.ID] = true
	}
	for _, entry := range audit {
		if entry.Path == "" || entry.Function == "" || entry.Rationale == "" || entry.MigrationDecision == "" {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("audit entry is incomplete: %+v", entry)})
		}
		if len(entry.AffectedRules) == 0 || len(entry.RequiredEvidence) == 0 {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("audit entry missing affected rules or evidence: %+v", entry)})
		}
		if !validPlane(entry.EvaluationPlane) {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("audit entry has invalid plane: %+v", entry)})
		}
		switch entry.Classification {
		case AuditGovernedExemption:
			if !registryIDs[entry.RegistryID] {
				issues = append(issues, CheckIssue{Message: fmt.Sprintf("audit entry references unknown registry ID %q", entry.RegistryID)})
			}
			governedIDs[entry.RegistryID] = true
		case AuditNonExemption, AuditSeparatePath:
			if entry.RegistryID != "" {
				issues = append(issues, CheckIssue{Message: fmt.Sprintf("non-governed audit entry references registry ID %q", entry.RegistryID)})
			}
		default:
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("audit entry has unknown classification %q", entry.Classification)})
		}
	}
	for id := range registryIDs {
		if !governedIDs[id] {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: registry entry has no governed audit inventory row", id)})
		}
	}
	return issues
}

func checkProductionReferences(root string, entries []Entry, audit []AuditEntry) []CheckIssue {
	registryIDs := map[string]bool{}
	for _, entry := range entries {
		registryIDs[entry.ID] = true
	}
	governedCallsites := map[string]AuditEntry{}
	for _, entry := range audit {
		if entry.Classification == AuditGovernedExemption {
			governedCallsites[entry.RegistryID] = entry
		}
	}

	var issues []CheckIssue
	for id, auditEntry := range governedCallsites {
		constName := constNameForID(id)
		if constName == "" {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: no exported registry constant maps to ID", id)})
			continue
		}
		found := false
		for _, path := range strings.Fields(auditEntry.Path) {
			b, err := os.ReadFile(filepath.Join(root, path))
			if err != nil {
				issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: cannot read production callsite %s: %v", id, path, err)})
				continue
			}
			if strings.Contains(string(b), constName) {
				found = true
			}
		}
		if !found {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s: no production callsite references %s", id, constName)})
		}
	}

	refs, err := productionRegistryRefs(root)
	if err != nil {
		issues = append(issues, CheckIssue{Message: fmt.Sprintf("cannot scan production registry references: %v", err)})
		return issues
	}
	for _, ref := range refs {
		if _, ok := registryIDs[idForConstName(ref.ConstName)]; !ok {
			issues = append(issues, CheckIssue{Message: fmt.Sprintf("%s references unknown registry constant %s", ref.Path, ref.ConstName)})
		}
	}
	return issues
}

type registryRef struct {
	Path      string
	ConstName string
}

func productionRegistryRefs(root string) ([]registryRef, error) {
	re := regexp.MustCompile(`exemptions\.([A-Za-z0-9_]+ID)`)
	var refs []registryRef
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "exemptions" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, match := range re.FindAllSubmatch(b, -1) {
			refs = append(refs, registryRef{Path: rel, ConstName: string(match[1])})
		}
		return nil
	})
	return refs, err
}

func constNameForID(id string) string {
	for name, candidate := range registryConstIDs() {
		if candidate == id {
			return name
		}
	}
	return ""
}

func idForConstName(name string) string {
	return registryConstIDs()[name]
}

func registryConstIDs() map[string]string {
	return map[string]string{
		"API001LiveEventsID":                      API001LiveEventsID,
		"API001AutoManagedFlowControlID":          API001AutoManagedFlowControlID,
		"API001ControllerManagedEndpointSlicesID": API001ControllerManagedEndpointSlicesID,
		"AddonProviderScopedCatalogID":            AddonProviderScopedCatalogID,
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

func (e Entry) allTests() []string {
	var out []string
	out = append(out, e.PositiveTests...)
	out = append(out, e.NegativeTests...)
	out = append(out, e.SpoofingTests...)
	out = append(out, e.PlaneTests...)
	return out
}
