package comparison

import (
	"sort"

	"kubepreflight/internal/findings"
)

// severityRank orders Blocker before Warning before Info -- plain
// alphabetical order would put Blocker before Info before Warning, which
// is wrong for a severity-first reading.
var severityRank = map[findings.Severity]int{
	findings.SeverityBlocker: 0,
	findings.SeverityWarning: 1,
	findings.SeverityInfo:    2,
}

// sortComparison orders every bucket deterministically so JSON/Markdown
// output is stable and golden-testable:
//
//   - New and Resolved: Blocker-severity entries first, then everything
//     else, each half ordered by the shared secondary chain below.
//   - Changed and Unchanged: the shared secondary chain directly, no
//     blocker-first split.
//
// Secondary chain: Priority, Severity, RuleID, Namespace, Resource name,
// Fingerprint (always unique, so it's the final tiebreaker -- input order
// never affects output).
func sortComparison(c *Comparison) {
	sort.SliceStable(c.New, func(i, j int) bool { return lessBlockerFirst(c.New[i].Finding, c.New[j].Finding) })
	sort.SliceStable(c.Resolved, func(i, j int) bool { return lessBlockerFirst(c.Resolved[i].Finding, c.Resolved[j].Finding) })
	sort.SliceStable(c.Unchanged, func(i, j int) bool { return sortKey(c.Unchanged[i].Finding).less(sortKey(c.Unchanged[j].Finding)) })
	sort.SliceStable(c.Changed, func(i, j int) bool { return changedSortKey(c.Changed[i]).less(changedSortKey(c.Changed[j])) })
}

func lessBlockerFirst(a, b findings.Finding) bool {
	aBlocker := a.Severity == findings.SeverityBlocker
	bBlocker := b.Severity == findings.SeverityBlocker
	if aBlocker != bBlocker {
		return aBlocker
	}
	return sortKey(a).less(sortKey(b))
}

// entrySortKey is the ordered tuple every bucket sorts on: Priority,
// Severity, RuleID, Namespace, Resource name, Fingerprint. Comparing it
// field-by-field (rather than OR-ing partial less-than results together)
// is what makes this a correct multi-key sort.
type entrySortKey struct {
	priority    string
	severityRnk int
	ruleID      string
	namespace   string
	name        string
	fingerprint string
}

func sortKey(f findings.Finding) entrySortKey {
	ns, name := firstResourceIdentity(f.Resources)
	return entrySortKey{
		priority:    priorityForSort(f.Priority),
		severityRnk: severityRank[f.Severity],
		ruleID:      f.RuleID,
		namespace:   ns,
		name:        name,
		fingerprint: f.Fingerprint,
	}
}

func changedSortKey(c Changed) entrySortKey {
	ns, name := firstResourceIdentity(c.Resources)
	priority := ""
	if fc, ok := c.Changes["priority"]; ok {
		priority = fc.After
	}
	return entrySortKey{
		priority:    priorityForSort(priority),
		ruleID:      c.RuleID,
		namespace:   ns,
		name:        name,
		fingerprint: c.Fingerprint,
	}
}

// priorityForSort maps "" to a value that sorts after every real priority
// ("P1".."P4" already sort correctly as plain strings), so a Changed entry
// with no priority field change still orders deterministically instead of
// sorting before P1 the way an empty string would.
func priorityForSort(p string) string {
	if p == "" {
		return "P9"
	}
	return p
}

func (k entrySortKey) less(other entrySortKey) bool {
	if k.priority != other.priority {
		return k.priority < other.priority
	}
	if k.severityRnk != other.severityRnk {
		return k.severityRnk < other.severityRnk
	}
	if k.ruleID != other.ruleID {
		return k.ruleID < other.ruleID
	}
	if k.namespace != other.namespace {
		return k.namespace < other.namespace
	}
	if k.name != other.name {
		return k.name < other.name
	}
	return k.fingerprint < other.fingerprint
}

func firstResourceIdentity(refs []findings.ResourceReference) (namespace, name string) {
	if len(refs) == 0 {
		return "", ""
	}
	return refs[0].Namespace, refs[0].Name
}
