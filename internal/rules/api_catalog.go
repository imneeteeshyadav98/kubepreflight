package rules

import "kubepreflight/internal/apicatalog"

// apiRemovalDecision is the removed-version fact API-001/API-002 evaluate a
// target version against, together with whether that fact came from the
// reviewed apicatalog.VersionedCatalog (source/reference attached) or from
// the legacy static apicatalog.Deprecated list the collector already used
// to discover this object.
//
// A missing or out-of-range versioned-catalog entry is never treated as
// "compatible" — resolveAPIRemoval falls back to the legacy removed-version
// fact, which is always defined for anything the collector already
// reported.
type apiRemovalDecision struct {
	RemovedInVersion string
	CatalogVerified  bool
	Source           string
	Reference        string
}

// resolveAPIRemoval prefers a reviewed apicatalog.VersionedCatalog entry for
// group/version/kind at targetVersion; when the catalog has no entry at
// all, or targetVersion falls outside that entry's verified supported
// range, it falls back to staticRemovedInVersion — the same
// apicatalog.Deprecated-sourced fact API-001/API-002 always used, so an
// unlisted or out-of-range API version never silently reads as compatible.
func resolveAPIRemoval(group, version, kind, targetVersion, staticRemovedInVersion string) apiRemovalDecision {
	vc, err := apicatalog.DefaultVersioned()
	if err == nil && vc != nil {
		if entry, ok := vc.Lookup(group, version, kind, targetVersion); ok {
			return apiRemovalDecision{
				RemovedInVersion: entry.RemovedInVersion,
				CatalogVerified:  true,
				Source:           entry.Source,
				Reference:        entry.Reference,
			}
		}
	}
	return apiRemovalDecision{RemovedInVersion: staticRemovedInVersion}
}

// evidence returns the extra evidence lines a catalog-verified decision
// contributes, or nil when the decision fell back to the legacy static
// list.
func (d apiRemovalDecision) evidence() []string {
	if !d.CatalogVerified {
		return nil
	}
	return []string{
		"catalog source: " + d.Source,
		"catalog reference: " + d.Reference,
	}
}
